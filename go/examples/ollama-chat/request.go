package main

import (
	"bytes"
	"encoding/json"

	sdk "github.com/bootun/legate-driver-sdk/go/driver"
	chat "github.com/bootun/legate-driver-sdk/go/protocol/openai/chatcompletions/v20260718"
)

type requestProblem struct {
	pointer string
	code    string
	message string
}

func problem(pointer, code, message string) *requestProblem {
	return &requestProblem{pointer: pointer, code: code, message: message}
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func convertOpenAIRequest(request chat.Request, upstreamModel string, mode sdk.TextMode) ([]byte, *requestProblem) {
	encodedMessages, err := json.Marshal(request.Messages)
	if err != nil {
		return nil, problem("/messages", "invalid", "messages could not be encoded")
	}
	messages, requestProblem := convertMessages(encodedMessages)
	if requestProblem != nil {
		return nil, requestProblem
	}
	encodedMessages, err = json.Marshal(messages)
	if err != nil {
		return nil, problem("/messages", "invalid", "messages could not be encoded")
	}

	output := cloneRawFields(request.ExtraFields)
	output["model"] = mustJSON(upstreamModel)
	output["messages"] = encodedMessages
	output["stream"] = mustJSON(mode == sdk.TextModeSSE)

	options, requestProblem := mergeOptions(request.ExtraFields)
	if requestProblem != nil {
		return nil, requestProblem
	}
	if len(options) > 0 {
		encodedOptions, err := json.Marshal(options)
		if err != nil {
			return nil, problem("/options", "invalid", "options could not be encoded")
		}
		output["options"] = encodedOptions
	}
	for _, name := range []string{"temperature", "top_p", "seed", "stop", "max_tokens", "max_completion_tokens"} {
		delete(output, name)
	}

	if raw, exists := request.ExtraFields["n"]; exists {
		var n int
		if json.Unmarshal(raw, &n) != nil || n != 1 {
			return nil, problem("/n", "unsupported", "Ollama supports exactly one choice")
		}
		delete(output, "n")
	}
	if raw, exists := request.ExtraFields["tool_choice"]; exists {
		var choice string
		if json.Unmarshal(raw, &choice) != nil || (choice != "auto" && choice != "none") {
			return nil, problem("/tool_choice", "unsupported", "tool_choice must be auto or none")
		}
		if choice == "none" {
			delete(output, "tools")
		}
		delete(output, "tool_choice")
	}
	if raw, exists := request.ExtraFields["parallel_tool_calls"]; exists {
		var enabled bool
		if json.Unmarshal(raw, &enabled) != nil || !enabled {
			return nil, problem("/parallel_tool_calls", "unsupported", "Ollama cannot disable parallel tool calls")
		}
		delete(output, "parallel_tool_calls")
	}
	if raw, exists := request.ExtraFields["response_format"]; exists {
		format, requestProblem := convertResponseFormat(raw)
		if requestProblem != nil {
			return nil, requestProblem
		}
		if len(format) > 0 {
			output["format"] = format
		}
		delete(output, "response_format")
	}
	if raw, exists := request.ExtraFields["reasoning_effort"]; exists {
		if _, conflict := request.ExtraFields["think"]; conflict {
			return nil, problem("/reasoning_effort", "conflict", "reasoning_effort and think cannot both be set")
		}
		var effort string
		if json.Unmarshal(raw, &effort) != nil || (effort != "low" && effort != "medium" && effort != "high") {
			return nil, problem("/reasoning_effort", "unsupported", "reasoning_effort must be low, medium, or high")
		}
		output["think"] = mustJSON(effort)
		delete(output, "reasoning_effort")
	}
	if raw, exists := request.ExtraFields["stream_options"]; exists {
		if !jsonObject(raw) {
			return nil, problem("/stream_options", "invalid_type", "stream_options must be an object")
		}
		delete(output, "stream_options")
	}
	for _, name := range []string{"user", "store", "metadata", "service_tier"} {
		delete(output, name)
	}
	for _, name := range []string{"frequency_penalty", "presence_penalty", "logit_bias", "prediction", "modalities", "audio", "web_search_options", "functions", "function_call"} {
		if _, exists := request.ExtraFields[name]; exists {
			return nil, problem("/"+name, "unsupported", name+" is not supported by Ollama Chat")
		}
	}

	body, err := json.Marshal(output)
	if err != nil {
		return nil, problem("", "invalid", "request could not be encoded")
	}
	return body, nil
}

func cloneRawFields(input map[string]json.RawMessage) map[string]json.RawMessage {
	output := make(map[string]json.RawMessage, len(input)+3)
	for name, value := range input {
		output[name] = append(json.RawMessage(nil), value...)
	}
	return output
}

func streamIncludesUsage(fields map[string]json.RawMessage) bool {
	raw, exists := fields["stream_options"]
	if !exists {
		return false
	}
	options, ok := decodeObject(raw)
	if !ok {
		return false
	}
	var include bool
	return json.Unmarshal(options["include_usage"], &include) == nil && include
}

func mustJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func mergeOptions(fields map[string]json.RawMessage) (map[string]json.RawMessage, *requestProblem) {
	options := make(map[string]json.RawMessage)
	if raw, exists := fields["options"]; exists {
		if !jsonObject(raw) || json.Unmarshal(raw, &options) != nil || options == nil {
			return nil, problem("/options", "invalid_type", "options must be an object")
		}
	}
	if _, oldExists := fields["max_tokens"]; oldExists {
		if _, newExists := fields["max_completion_tokens"]; newExists {
			return nil, problem("/max_completion_tokens", "conflict", "max_tokens and max_completion_tokens cannot both be set")
		}
	}
	for _, mapping := range []struct{ openAI, ollama string }{
		{openAI: "temperature", ollama: "temperature"},
		{openAI: "top_p", ollama: "top_p"},
		{openAI: "seed", ollama: "seed"},
		{openAI: "stop", ollama: "stop"},
		{openAI: "max_tokens", ollama: "num_predict"},
		{openAI: "max_completion_tokens", ollama: "num_predict"},
	} {
		if raw, exists := fields[mapping.openAI]; exists {
			options[mapping.ollama] = append(json.RawMessage(nil), raw...)
		}
	}
	return options, nil
}

func convertResponseFormat(raw json.RawMessage) (json.RawMessage, *requestProblem) {
	fields, ok := decodeObject(raw)
	if !ok {
		return nil, problem("/response_format", "invalid_type", "response_format must be an object")
	}
	formatType, requestProblem := requiredString(fields, "type", "/response_format/type")
	if requestProblem != nil {
		return nil, requestProblem
	}
	switch formatType {
	case "text":
		return nil, nil
	case "json_object":
		return mustJSON("json"), nil
	case "json_schema":
		rawSchema, exists := fields["json_schema"]
		if !exists {
			return nil, problem("/response_format/json_schema", "required", "json_schema is required")
		}
		schemaFields, ok := decodeObject(rawSchema)
		if !ok {
			return nil, problem("/response_format/json_schema", "invalid_type", "json_schema must be an object")
		}
		schema, exists := schemaFields["schema"]
		if !exists || !jsonObject(schema) {
			return nil, problem("/response_format/json_schema/schema", "required", "schema must be an object")
		}
		return append(json.RawMessage(nil), schema...), nil
	default:
		return nil, problem("/response_format/type", "unsupported", "response format is not supported")
	}
}

func convertMessages(raw json.RawMessage) ([]ollamaMessage, *requestProblem) {
	var encoded []json.RawMessage
	if json.Unmarshal(raw, &encoded) != nil || encoded == nil {
		return nil, problem("/messages", "invalid_type", "messages must be an array")
	}
	messages := make([]ollamaMessage, 0, len(encoded))
	toolNames := make(map[string]string)
	for index, encodedMessage := range encoded {
		pointer := "/messages/" + decimal(index)
		fields, ok := decodeObject(encodedMessage)
		if !ok {
			return nil, problem(pointer, "invalid_type", "message must be an object")
		}
		role, requestProblem := requiredString(fields, "role", pointer+"/role")
		if requestProblem != nil {
			return nil, requestProblem
		}
		if role == "developer" {
			role = "system"
		}
		if role != "system" && role != "user" && role != "assistant" && role != "tool" {
			return nil, problem(pointer+"/role", "unsupported", "message role is not supported")
		}
		content, images, requestProblem := convertMessageContent(fields["content"], pointer+"/content")
		if requestProblem != nil {
			return nil, requestProblem
		}
		message := ollamaMessage{Role: role, Content: content, Images: images}
		if rawCalls, exists := fields["tool_calls"]; exists {
			calls, names, requestProblem := convertInboundToolCalls(rawCalls, pointer+"/tool_calls")
			if requestProblem != nil {
				return nil, requestProblem
			}
			message.ToolCalls = calls
			for id, name := range names {
				toolNames[id] = name
			}
		}
		if role == "tool" {
			if rawID, exists := fields["tool_call_id"]; exists {
				var id string
				if json.Unmarshal(rawID, &id) != nil || id == "" {
					return nil, problem(pointer+"/tool_call_id", "invalid_type", "tool_call_id must be a non-empty string")
				}
				message.ToolName = toolNames[id]
			}
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func convertMessageContent(raw json.RawMessage, pointer string) (string, []string, *requestProblem) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil, nil
	}
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil || parts == nil {
		return "", nil, problem(pointer, "invalid_type", "content must be a string or content-part array")
	}
	var content bytes.Buffer
	var images []string
	for index, rawPart := range parts {
		partPointer := pointer + "/" + decimal(index)
		fields, ok := decodeObject(rawPart)
		if !ok {
			return "", nil, problem(partPointer, "invalid_type", "content part must be an object")
		}
		partType, requestProblem := requiredString(fields, "type", partPointer+"/type")
		if requestProblem != nil {
			return "", nil, requestProblem
		}
		switch partType {
		case "text":
			value, requestProblem := requiredString(fields, "text", partPointer+"/text")
			if requestProblem != nil {
				return "", nil, requestProblem
			}
			content.WriteString(value)
		case "image_url":
			imageFields, ok := decodeObject(fields["image_url"])
			if !ok {
				return "", nil, problem(partPointer+"/image_url", "invalid_type", "image_url must be an object")
			}
			url, requestProblem := requiredString(imageFields, "url", partPointer+"/image_url/url")
			if requestProblem != nil {
				return "", nil, requestProblem
			}
			comma := bytes.IndexByte([]byte(url), ',')
			if comma < 0 || !bytes.Contains([]byte(url[:comma]), []byte(";base64")) || len(url) == comma+1 {
				return "", nil, problem(partPointer+"/image_url/url", "unsupported", "image URL must be a base64 data URL")
			}
			images = append(images, url[comma+1:])
		default:
			return "", nil, problem(partPointer+"/type", "unsupported", "content part type is not supported")
		}
	}
	return content.String(), images, nil
}

func convertInboundToolCalls(raw json.RawMessage, pointer string) ([]ollamaToolCall, map[string]string, *requestProblem) {
	var encoded []json.RawMessage
	if json.Unmarshal(raw, &encoded) != nil || encoded == nil {
		return nil, nil, problem(pointer, "invalid_type", "tool_calls must be an array")
	}
	calls := make([]ollamaToolCall, 0, len(encoded))
	names := make(map[string]string)
	for index, rawCall := range encoded {
		callPointer := pointer + "/" + decimal(index)
		fields, ok := decodeObject(rawCall)
		if !ok {
			return nil, nil, problem(callPointer, "invalid_type", "tool call must be an object")
		}
		if rawType, exists := fields["type"]; exists {
			var callType string
			if json.Unmarshal(rawType, &callType) != nil || callType != "function" {
				return nil, nil, problem(callPointer+"/type", "unsupported", "only function tool calls are supported")
			}
		}
		functionFields, ok := decodeObject(fields["function"])
		if !ok {
			return nil, nil, problem(callPointer+"/function", "invalid_type", "function must be an object")
		}
		name, requestProblem := requiredString(functionFields, "name", callPointer+"/function/name")
		if requestProblem != nil || name == "" {
			return nil, nil, problem(callPointer+"/function/name", "required", "function name is required")
		}
		var argumentsText string
		if json.Unmarshal(functionFields["arguments"], &argumentsText) != nil {
			return nil, nil, problem(callPointer+"/function/arguments", "invalid_type", "function arguments must be a JSON string")
		}
		arguments := json.RawMessage(argumentsText)
		if !jsonObject(arguments) {
			return nil, nil, problem(callPointer+"/function/arguments", "invalid_json_object", "function arguments must encode a JSON object")
		}
		calls = append(calls, ollamaToolCall{Function: ollamaToolFunction{Name: name, Arguments: arguments}})
		var id string
		if json.Unmarshal(fields["id"], &id) == nil && id != "" {
			names[id] = name
		}
	}
	return calls, names, nil
}
