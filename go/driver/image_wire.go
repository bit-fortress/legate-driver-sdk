package driver

import "math"

func decodeImageAttemptOpenRequest(data []byte) (ImageAttemptOpenRequest, error) {
	var result ImageAttemptOpenRequest
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
			invocation, err := decodeImageInvocation(value)
			if err != nil {
				return result, err
			}
			result.Invocation = &invocation
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeImageInvocation(data []byte) (ImageInvocation, error) {
	var result ImageInvocation
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
			request, err := decodeImageProtocolRequest(value)
			if err != nil {
				return result, err
			}
			result.Request = &request
		case 2, 3:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			text, err := strictString(value)
			if err != nil {
				return result, err
			}
			if field == 2 {
				result.SelectedUpstreamModel = text
			} else {
				result.ResponseID = text
			}
		case 4:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			ref, err := decodeBlobRef(value)
			if err != nil {
				return result, err
			}
			result.Blobs = append(result.Blobs, ref)
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeImageProtocolRequest(data []byte) (ImageProtocolRequest, error) {
	var result ImageProtocolRequest
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1, 2:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			text, err := strictString(value)
			if err != nil {
				return result, err
			}
			if field == 1 {
				result.ProtocolContract = text
			} else {
				result.MediaType = text
			}
		case 3:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.JSON = clonePresentBytes(value)
			result.Multipart = nil
		case 4:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			multipart, err := decodeMultipartInput(value)
			if err != nil {
				return result, err
			}
			result.Multipart = &multipart
			result.JSON = nil
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeMultipartInput(data []byte) (MultipartInput, error) {
	var result MultipartInput
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		if field != 1 {
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
			continue
		}
		value, err := d.bytes(wire)
		if err != nil {
			return result, err
		}
		part, err := decodeMultipartInputPart(value)
		if err != nil {
			return result, err
		}
		result.Parts = append(result.Parts, part)
	}
	return result, nil
}

func decodeMultipartInputPart(data []byte) (MultipartInputPart, error) {
	var result MultipartInputPart
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1, 2, 4:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			text, err := strictString(value)
			if err != nil {
				return result, err
			}
			switch field {
			case 1:
				result.Name = text
			case 2:
				result.Filename = &text
			case 4:
				result.ContentType = text
			}
		case 3:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			header, err := decodeNameValues(value)
			if err != nil {
				return result, err
			}
			result.Headers = append(result.Headers, header)
		case 5:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.Inline = clonePresentBytes(value)
			result.Blob = nil
		case 6:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			ref, err := decodeBlobRef(value)
			if err != nil {
				return result, err
			}
			result.Blob = &ref
			result.Inline = nil
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeBlobRef(data []byte) (BlobRef, error) {
	var result BlobRef
	d := decoder{data: data}
	for d.pos < len(data) {
		field, wire, err := d.key()
		if err != nil {
			return result, err
		}
		switch field {
		case 1, 2:
			value, err := d.uint64(wire)
			if err != nil || field == 2 && value > math.MaxInt64 {
				return result, errInvalidWire
			}
			if field == 1 {
				result.ID = value
			} else {
				result.Size = int64(value)
			}
		case 3:
			value, err := d.bytes(wire)
			if err != nil || len(value) != len(result.SHA256) {
				return result, errInvalidWire
			}
			copy(result.SHA256[:], value)
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeImageTransformBufferedResponseRequest(data []byte) (ImageTransformBufferedResponseRequest, error) {
	var result ImageTransformBufferedResponseRequest
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
				var upstream ImageUpstreamResponse
				upstream, err = decodeImageUpstreamResponse(value)
				result.Upstream = &upstream
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

func decodeImageUpstreamResponse(data []byte) (ImageUpstreamResponse, error) {
	var result ImageUpstreamResponse
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
			head, err := decodeUpstreamResponseHead(value)
			if err != nil {
				return result, err
			}
			result.Head = &head
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			body, err := decodeImageUpstreamBody(value)
			if err != nil {
				return result, err
			}
			result.Body = &body
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeImageUpstreamBody(data []byte) (ImageUpstreamBody, error) {
	var result ImageUpstreamBody
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
			result.MediaType, err = strictString(value)
			if err != nil {
				return result, err
			}
		case 2:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			result.Inline = clonePresentBytes(value)
			result.Blob = nil
		case 3:
			value, err := d.bytes(wire)
			if err != nil {
				return result, err
			}
			ref, err := decodeBlobRef(value)
			if err != nil {
				return result, err
			}
			result.Blob = &ref
			result.Inline = nil
		default:
			if err := d.skip(field, wire); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func decodeImageAttemptCloseRequest(data []byte) (ImageAttemptCloseRequest, error) {
	handle, err := decodeHandleOnlyRequest(data)
	return ImageAttemptCloseRequest{AttemptHandle: handle}, err
}

func encodeBlobRef(value BlobRef) []byte {
	var output []byte
	output = appendUint64(output, 1, value.ID)
	output = appendUint64(output, 2, uint64(value.Size))
	return appendBytes(output, 3, value.SHA256[:])
}

func encodeBodySegment(value BodySegment) []byte {
	switch value.Kind {
	case BodySourceInline:
		return appendBytes(nil, 1, value.Inline)
	case BodySourceBlob:
		return appendMessage(nil, 2, encodeBlobRef(*value.Blob))
	case BodySourceBase64:
		return appendMessage(nil, 3, encodeBlobRef(*value.Blob))
	default:
		return nil
	}
}

func encodeBodyPlan(value BodyPlan) []byte {
	output := appendString(nil, 1, value.MediaType)
	switch value.Kind {
	case BodyPlanInline:
		output = appendMessage(output, 2, appendBytes(nil, 1, value.Inline))
	case BodyPlanBlob:
		output = appendMessage(output, 3, encodeBlobRef(*value.Blob))
	case BodyPlanBase64:
		output = appendMessage(output, 4, encodeBlobRef(*value.Blob))
	case BodyPlanComposite:
		var composite []byte
		for _, segment := range value.Segments {
			composite = appendMessage(composite, 1, encodeBodySegment(segment))
		}
		output = appendMessage(output, 5, composite)
	case BodyPlanMultipart:
		var multipart []byte
		for _, part := range value.Parts {
			var nested []byte
			nested = appendString(nested, 1, part.Name)
			if part.Filename != nil {
				nested = appendString(nested, 2, *part.Filename)
			}
			for _, header := range part.Headers {
				nested = appendMessage(nested, 3, encodeNameValues(header))
			}
			nested = appendString(nested, 4, part.ContentType)
			nested = appendMessage(nested, 5, encodeBodySegment(part.Content))
			multipart = appendMessage(multipart, 1, nested)
		}
		output = appendMessage(output, 6, multipart)
	}
	return output
}

func encodeImageClientResponse(value ImageClientResponse) []byte {
	var output []byte
	output = appendEnum(output, 1, value.StatusCode)
	for _, header := range value.Headers {
		output = appendMessage(output, 2, encodeNameValues(header))
	}
	output = appendString(output, 3, value.ProtocolContract)
	if value.Body != nil {
		output = appendMessage(output, 4, encodeBodyPlan(*value.Body))
	}
	if value.Outcome != nil {
		output = appendMessage(output, 5, encodeSemanticOutcome(*value.Outcome))
	}
	if value.Usage != nil {
		output = appendMessage(output, 6, encodeUsageReport(*value.Usage))
	}
	return output
}

func encodeImageAttemptOpenResponse(success *ImageAttemptOpenSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	nested := appendUint64(nil, 1, success.AttemptHandle)
	if success.Request != nil {
		nested = appendMessage(nested, 2, encodeRequestPlan(*success.Request))
	}
	if success.Response != nil {
		nested = appendMessage(nested, 3, encodeImageClientResponse(*success.Response))
	}
	return appendMessage(nil, 1, nested)
}

func encodeImageTransformBufferedResponseResponse(success *ImageTransformBufferedResponseSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	var nested []byte
	if success.Response != nil {
		nested = appendMessage(nested, 1, encodeImageClientResponse(*success.Response))
	}
	return appendMessage(nil, 1, nested)
}

func encodeImageAttemptCloseResponse(success *ImageAttemptCloseSuccess, failure *DriverError) []byte {
	if success == nil {
		return appendMessage(nil, 2, encodeDriverError(failure))
	}
	return appendMessage(nil, 1, nil)
}

func clonePresentBytes(value []byte) []byte { return append([]byte{}, value...) }
