package driver

import (
	"errors"
	"unicode/utf8"
)

var errInvalidWire = errors.New("invalid driver ABI message")

type decoder struct {
	data []byte
	pos  int
}

func (d *decoder) key() (int, int, error) {
	value, err := d.varint()
	if err != nil {
		return 0, 0, errInvalidWire
	}
	field := value >> 3
	wire := value & 7
	if field == 0 || field > 1<<29-1 || wire > 5 {
		return 0, 0, errInvalidWire
	}
	return int(field), int(wire), nil
}

func (d *decoder) varint() (uint64, error) {
	var value uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if d.pos >= len(d.data) {
			return 0, errInvalidWire
		}
		current := d.data[d.pos]
		d.pos++
		if shift == 63 && current > 1 {
			return 0, errInvalidWire
		}
		value |= uint64(current&0x7f) << shift
		if current < 0x80 {
			return value, nil
		}
	}
	return 0, errInvalidWire
}

func (d *decoder) bytes(wire int) ([]byte, error) {
	if wire != 2 {
		return nil, errInvalidWire
	}
	length, err := d.varint()
	if err != nil || length > uint64(len(d.data)-d.pos) {
		return nil, errInvalidWire
	}
	start := d.pos
	end := start + int(length)
	d.pos = end
	return d.data[start:end], nil
}

func (d *decoder) uint64(wire int) (uint64, error) {
	if wire != 0 {
		return 0, errInvalidWire
	}
	return d.varint()
}

func (d *decoder) skip(field int, wire int) error {
	return d.skipAt(field, wire, 0)
}

func (d *decoder) skipAt(field int, wire int, depth int) error {
	switch wire {
	case 0:
		_, err := d.varint()
		return err
	case 1:
		if d.pos+8 > len(d.data) {
			return errInvalidWire
		}
		d.pos += 8
	case 2:
		_, err := d.bytes(wire)
		return err
	case 3:
		if depth >= 100 {
			return errInvalidWire
		}
		for d.pos < len(d.data) {
			nestedField, nestedWire, err := d.key()
			if err != nil {
				return err
			}
			if nestedWire == 4 {
				if nestedField != field {
					return errInvalidWire
				}
				return nil
			}
			if err := d.skipAt(nestedField, nestedWire, depth+1); err != nil {
				return err
			}
		}
		return errInvalidWire
	case 4:
		return errInvalidWire
	case 5:
		if d.pos+4 > len(d.data) {
			return errInvalidWire
		}
		d.pos += 4
	default:
		return errInvalidWire
	}
	return nil
}

func decodeBindRequest(data []byte) (BindRequest, error) {
	var result BindRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.ConfigJSON = cloneBytes(value)
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			decoded, err := strictString(value)
			if err != nil {
				return result, err
			}
			result.EndpointRefs = append(result.EndpointRefs, decoded)
		case 3:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			slot, err := decodeCredentialSlot(value)
			if err != nil {
				return result, err
			}
			result.CredentialSlots = append(result.CredentialSlots, slot)
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeCredentialSlot(data []byte) (CredentialSlotDescriptor, error) {
	var result CredentialSlotDescriptor
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.Name, err = strictString(value)
			if err != nil {
				return result, err
			}
		case 2:
			value, err := d.uint64(wire)
			if err != nil {
				return result, err
			}
			result.Configured = value != 0
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeTextAttemptOpenRequest(data []byte) (TextAttemptOpenRequest, error) {
	var result TextAttemptOpenRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.BoundState = cloneBytes(value)
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			if result.Invocation == nil {
				result.Invocation = &TextInvocation{}
			}
			if err := decodeTextInvocationInto(result.Invocation, value); err != nil {
				return result, err
			}
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeTextInvocation(data []byte) (TextInvocation, error) {
	var result TextInvocation
	err := decodeTextInvocationInto(&result, data)
	return result, err
}

func decodeTextInvocationInto(result *TextInvocation, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		switch field {
		case 1:
			value, err := d.uint64(wire)
			if err != nil {
				return err
			}
			result.Mode = TextMode(value)
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			if result.Request == nil {
				result.Request = &ProtocolPayload{}
			}
			if err := decodeProtocolPayloadInto(result.Request, value); err != nil {
				return err
			}
		case 3:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			if result.ProtocolMetadata == nil {
				result.ProtocolMetadata = &ProtocolPayload{}
			}
			err = decodeProtocolPayloadInto(result.ProtocolMetadata, value)
			if err != nil {
				return err
			}
		case 4, 5:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			decoded, err := strictString(value)
			if err != nil {
				return err
			}
			if field == 4 {
				result.SelectedUpstreamModel = decoded
			} else {
				result.ResponseID = decoded
			}
		default:
			if err := d.skip(field, wire); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeProtocolPayload(data []byte) (ProtocolPayload, error) {
	var result ProtocolPayload
	err := decodeProtocolPayloadInto(&result, data)
	return result, err
}

func decodeProtocolPayloadInto(result *ProtocolPayload, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		switch field {
		case 1, 2, 3:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			var decoded string
			if field != 3 {
				decoded, err = strictString(value)
				if err != nil {
					return err
				}
			}
			switch field {
			case 1:
				result.ProtocolContract = decoded
			case 2:
				result.MediaType = decoded
			case 3:
				result.JSON = cloneBytes(value)
			}
		default:
			if err := d.skip(field, wire); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeTextTransformBufferedResponseRequest(data []byte) (TextTransformBufferedResponseRequest, error) {
	var result TextTransformBufferedResponseRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			result.AttemptHandle, err = d.uint64(wire)
		case 2:
			var value []byte
			value, err = d.bytes(wire)
			if err == nil {
				if result.Upstream == nil {
					result.Upstream = &BufferedUpstreamResponse{}
				}
				err = decodeBufferedUpstreamResponseInto(result.Upstream, value)
			}
		default:
			err = d.skip(field, wire)
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func decodeBufferedUpstreamResponse(data []byte) (BufferedUpstreamResponse, error) {
	var result BufferedUpstreamResponse
	err := decodeBufferedUpstreamResponseInto(&result, data)
	return result, err
}

func decodeBufferedUpstreamResponseInto(result *BufferedUpstreamResponse, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		switch field {
		case 1, 2:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			if field == 1 {
				if result.Head == nil {
					result.Head = &UpstreamResponseHead{}
				}
				err = decodeUpstreamResponseHeadInto(result.Head, value)
			} else {
				if result.Body == nil {
					result.Body = &MessageBody{}
				}
				err = decodeMessageBodyInto(result.Body, value)
			}
		default:
			err = d.skip(field, wire)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeUpstreamResponseHead(data []byte) (UpstreamResponseHead, error) {
	var result UpstreamResponseHead
	err := decodeUpstreamResponseHeadInto(&result, data)
	return result, err
}

func decodeUpstreamResponseHeadInto(result *UpstreamResponseHead, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		switch field {
		case 1:
			value, err := d.uint64(wire)
			if err != nil {
				return err
			}
			result.StatusCode = int32(value)
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			header, err := decodeNameValues(value)
			if err != nil {
				return err
			}
			result.Headers = append(result.Headers, header)
		default:
			if err := d.skip(field, wire); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeTextSSEOpenRequest(data []byte) (TextSSEOpenRequest, error) {
	var result TextSSEOpenRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			result.AttemptHandle, err = d.uint64(wire)
		case 2:
			var value []byte
			value, err = d.bytes(wire)
			if err == nil {
				if result.Upstream == nil {
					result.Upstream = &UpstreamResponseHead{}
				}
				err = decodeUpstreamResponseHeadInto(result.Upstream, value)
			}
		default:
			err = d.skip(field, wire)
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func decodeTextSSETransformEventRequest(data []byte) (TextSSETransformEventRequest, error) {
	var result TextSSETransformEventRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1:
			result.AttemptHandle, err = d.uint64(wire)
		case 2:
			var value []byte
			value, err = d.bytes(wire)
			if err == nil {
				if result.Upstream == nil {
					result.Upstream = &UpstreamSSEEvent{}
				}
				err = decodeUpstreamSSEEventInto(result.Upstream, value)
			}
		default:
			err = d.skip(field, wire)
		}
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func decodeUpstreamSSEEvent(data []byte) (UpstreamSSEEvent, error) {
	var result UpstreamSSEEvent
	err := decodeUpstreamSSEEventInto(&result, data)
	return result, err
}

func decodeUpstreamSSEEventInto(result *UpstreamSSEEvent, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		switch field {
		case 1, 2, 3:
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			switch field {
			case 1:
				result.EventType, err = strictString(value)
				if err != nil {
					return err
				}
			case 2:
				result.Data = cloneBytes(value)
			case 3:
				lastEventID, err := strictString(value)
				if err != nil {
					return err
				}
				result.LastEventID = &lastEventID
			}
		case 4:
			value, err := d.uint64(wire)
			if err != nil {
				return err
			}
			result.RetryMilliseconds = new(uint64)
			*result.RetryMilliseconds = value
		default:
			if err := d.skip(field, wire); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeTextSSEFinishRequest(data []byte) (TextSSEFinishRequest, error) {
	var result TextSSEFinishRequest
	handle, err := decodeHandleOnlyRequest(data)
	result.AttemptHandle = handle
	return result, err
}

func decodeTextAttemptCloseRequest(data []byte) (TextAttemptCloseRequest, error) {
	var result TextAttemptCloseRequest
	handle, err := decodeHandleOnlyRequest(data)
	result.AttemptHandle = handle
	return result, err
}

func decodeHandleOnlyRequest(data []byte) (uint64, error) {
	var handle uint64
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return 0, err
		}
		if field == 1 {
			handle, err = d.uint64(wire)
		} else {
			err = d.skip(field, wire)
		}
		if err != nil {
			return 0, err
		}
	}
	return handle, nil
}

func decodeNameValues(data []byte) (NameValues, error) {
	var result NameValues
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		if field == 1 || field == 2 {
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			if field == 1 {
				result.Name, err = strictString(value)
			} else {
				var decoded string
				decoded, err = strictString(value)
				result.Values = append(result.Values, decoded)
			}
			if err != nil {
				return result, err
			}
			continue
		}
		if err := d.skip(field, wire); err != nil {
			return result, err
		}
	}
	return result, nil
}

func decodeMessageBody(data []byte) (MessageBody, error) {
	var result MessageBody
	err := decodeMessageBodyInto(&result, data)
	return result, err
}

func decodeMessageBodyInto(result *MessageBody, data []byte) error {
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return err
		}
		if field == 1 || field == 2 {
			value, err := d.bytes(wire)
			if err != nil {
				return err
			}
			if field == 1 {
				result.MediaType, err = strictString(value)
				if err != nil {
					return err
				}
			} else {
				result.Payload = cloneBytes(value)
			}
			continue
		}
		if err := d.skip(field, wire); err != nil {
			return err
		}
	}
	return nil
}

func appendVarint(output []byte, value uint64) []byte {
	for value >= 0x80 {
		output = append(output, byte(value)|0x80)
		value >>= 7
	}
	return append(output, byte(value))
}

func appendKey(output []byte, field int, wire int) []byte {
	return appendVarint(output, uint64(field<<3|wire))
}

func appendBytes(output []byte, field int, value []byte) []byte {
	if len(value) == 0 {
		return output
	}
	return appendLengthDelimited(output, field, value)
}

func appendMessage(output []byte, field int, value []byte) []byte {
	return appendLengthDelimited(output, field, value)
}

func appendLengthDelimited(output []byte, field int, value []byte) []byte {
	output = appendKey(output, field, 2)
	output = appendVarint(output, uint64(len(value)))
	return append(output, value...)
}

func appendString(output []byte, field int, value string) []byte {
	return appendBytes(output, field, []byte(value))
}

func appendUint64(output []byte, field int, value uint64) []byte {
	if value == 0 {
		return output
	}
	output = appendKey(output, field, 0)
	return appendVarint(output, value)
}

func appendOptionalInt64(output []byte, field int, value *int64) []byte {
	if value == nil {
		return output
	}
	output = appendKey(output, field, 0)
	return appendVarint(output, uint64(*value))
}

func appendEnum(output []byte, field int, value int32) []byte {
	return appendUint64(output, field, uint64(value))
}

func appendPackedEnums(output []byte, field int, values []int32) []byte {
	if len(values) == 0 {
		return output
	}
	var packed []byte
	for _, value := range values {
		packed = appendVarint(packed, uint64(value))
	}
	return appendLengthDelimited(output, field, packed)
}

func encodeDriverError(value *DriverError) []byte {
	if value == nil {
		return nil
	}
	var output []byte
	output = appendString(output, 1, value.Code)
	output = appendString(output, 2, value.Message)
	for _, issue := range value.Issues {
		var nested []byte
		nested = appendString(nested, 1, issue.Pointer)
		nested = appendString(nested, 2, issue.Code)
		nested = appendString(nested, 3, issue.Message)
		output = appendMessage(output, 3, nested)
	}
	if value.Usage != nil {
		output = appendMessage(output, 4, encodeUsageReport(*value.Usage))
	}
	return output
}

func encodeNameValues(value NameValues) []byte {
	var output []byte
	output = appendString(output, 1, value.Name)
	for _, item := range value.Values {
		output = appendString(output, 2, item)
	}
	return output
}

func encodeMessageBody(value MessageBody) []byte {
	var output []byte
	output = appendString(output, 1, value.MediaType)
	return appendBytes(output, 2, value.Payload)
}

func encodeProtocolPayload(value ProtocolPayload) []byte {
	var output []byte
	output = appendString(output, 1, value.ProtocolContract)
	output = appendString(output, 2, value.MediaType)
	return appendBytes(output, 3, value.JSON)
}

func encodeProtocolEventPayload(value ProtocolEventPayload) []byte {
	var output []byte
	output = appendString(output, 1, value.ProtocolContract)
	output = appendString(output, 2, value.EventType)
	return appendBytes(output, 3, value.JSON)
}

func encodeTextCapabilities(value TextCapabilities) []byte {
	var output []byte
	for _, contract := range value.ProtocolContracts {
		output = appendString(output, 1, contract)
	}
	return output
}

func encodeImageCapabilities(value ImageCapabilities) []byte {
	var output []byte
	for _, contract := range value.ProtocolContracts {
		output = appendString(output, 1, contract)
	}
	return output
}

func encodeBindResponse(success *BindSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	nested = appendBytes(nested, 1, success.BoundState)
	if success.TextCapabilities != nil {
		nested = appendMessage(nested, 2, encodeTextCapabilities(*success.TextCapabilities))
	}
	if success.ImageCapabilities != nil {
		nested = appendMessage(nested, 3, encodeImageCapabilities(*success.ImageCapabilities))
	}
	return appendMessage(nil, 1, nested)
}

func encodeTextAttemptOpenResponse(success *TextAttemptOpenSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	nested := appendUint64(nil, 1, success.AttemptHandle)
	if success.Request != nil {
		nested = appendMessage(nested, 2, encodeRequestPlan(*success.Request))
	}
	if success.Response != nil {
		nested = appendMessage(nested, 3, encodeClientResponse(*success.Response))
	}
	return appendMessage(nil, 1, nested)
}

func encodeAuthPlan(value AuthPlan) []byte {
	var output []byte
	output = appendEnum(output, 1, int32(value.Kind))
	output = appendString(output, 2, value.CredentialSlot)
	return appendString(output, 3, value.HeaderName)
}

func encodeRequestPlan(value RequestPlan) []byte {
	var output []byte
	output = appendString(output, 1, value.EndpointRef)
	output = appendString(output, 2, value.Method)
	output = appendString(output, 3, value.RelativePath)
	for _, item := range value.Query {
		output = appendMessage(output, 4, encodeNameValues(item))
	}
	for _, item := range value.Headers {
		output = appendMessage(output, 5, encodeNameValues(item))
	}
	if value.Body != nil {
		output = appendMessage(output, 6, encodeMessageBody(*value.Body))
	}
	if value.Auth != nil {
		output = appendMessage(output, 7, encodeAuthPlan(*value.Auth))
	}
	if value.BodyPlan != nil {
		output = appendMessage(output, 8, encodeBodyPlan(*value.BodyPlan))
	}
	return output
}

func encodeSemanticOutcome(value SemanticOutcome) []byte {
	var output []byte
	output = appendEnum(output, 1, int32(value.Class))
	return appendString(output, 2, value.VendorCode)
}

func encodeUsageReport(value UsageReport) []byte {
	var output []byte
	output = appendEnum(output, 1, int32(value.Status))
	output = appendOptionalInt64(output, 2, value.InputTokens)
	output = appendOptionalInt64(output, 3, value.OutputTokens)
	output = appendOptionalInt64(output, 4, value.CachedTokens)
	output = appendOptionalInt64(output, 5, value.ReasoningTokens)
	return appendEnum(output, 6, int32(value.Provenance))
}

func encodeClientResponse(value ClientResponse) []byte {
	var output []byte
	output = appendEnum(output, 1, value.StatusCode)
	for _, header := range value.Headers {
		output = appendMessage(output, 2, encodeNameValues(header))
	}
	if value.Body != nil {
		output = appendMessage(output, 3, encodeProtocolPayload(*value.Body))
	}
	if value.Outcome != nil {
		output = appendMessage(output, 4, encodeSemanticOutcome(*value.Outcome))
	}
	if value.Usage != nil {
		output = appendMessage(output, 5, encodeUsageReport(*value.Usage))
	}
	return output
}

func encodeTextTransformBufferedResponseResponse(success *TextTransformBufferedResponseSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	if success.Response != nil {
		nested = appendMessage(nested, 1, encodeClientResponse(*success.Response))
	}
	return appendMessage(nil, 1, nested)
}

func encodeTextSSEOpenResponse(success *TextSSEOpenSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	nested = appendEnum(nested, 1, success.StatusCode)
	for _, header := range success.Headers {
		nested = appendMessage(nested, 2, encodeNameValues(header))
	}
	if success.Outcome != nil {
		nested = appendMessage(nested, 3, encodeSemanticOutcome(*success.Outcome))
	}
	return appendMessage(nil, 1, nested)
}

func encodeTextSSETransformEventResponse(success *TextSSETransformEventSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	for _, event := range success.Events {
		nested = appendMessage(nested, 1, encodeProtocolEventPayload(event))
	}
	if success.Outcome != nil {
		nested = appendMessage(nested, 2, encodeSemanticOutcome(*success.Outcome))
	}
	if success.Usage != nil {
		nested = appendMessage(nested, 3, encodeUsageReport(*success.Usage))
	}
	return appendMessage(nil, 1, nested)
}

func encodeTextSSEFinishResponse(success *TextSSEFinishSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	for _, event := range success.Events {
		nested = appendMessage(nested, 1, encodeProtocolEventPayload(event))
	}
	if success.Outcome != nil {
		nested = appendMessage(nested, 2, encodeSemanticOutcome(*success.Outcome))
	}
	if success.Usage != nil {
		nested = appendMessage(nested, 3, encodeUsageReport(*success.Usage))
	}
	return appendMessage(nil, 1, nested)
}

func encodeTextAttemptCloseResponse(success *TextAttemptCloseSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	return appendMessage(nil, 1, nil)
}

func cloneBytes(value []byte) []byte {
	return append([]byte(nil), value...)
}

func strictString(value []byte) (string, error) {
	if !utf8.Valid(value) {
		return "", errInvalidWire
	}
	return string(value), nil
}
