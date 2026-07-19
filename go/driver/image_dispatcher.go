package driver

import (
	"errors"
	"sort"
)

type ImageBinder interface {
	BindImage(BindRequest) ([]byte, *DriverError)
}

type ImageBinderFunc func(BindRequest) ([]byte, *DriverError)

func (f ImageBinderFunc) BindImage(request BindRequest) ([]byte, *DriverError) { return f(request) }

type imageProtocolOpenFunc func(ImageAttemptOpenRequest) (*ImageAttemptOpenResult, *DriverError)

type ImageProtocolHandlerRegistration struct {
	contract string
	open     imageProtocolOpenFunc
}

func NewImageProtocolHandlerRegistration(contract string, open imageProtocolOpenFunc) ImageProtocolHandlerRegistration {
	return ImageProtocolHandlerRegistration{contract: contract, open: open}
}

type ImageDispatcher struct {
	binder    ImageBinder
	contracts []string
	handlers  map[string]imageProtocolOpenFunc
}

func NewImageDispatcher(declaredContracts []string, binder ImageBinder, registrations ...ImageProtocolHandlerRegistration) (*ImageDispatcher, error) {
	if binder == nil || len(declaredContracts) == 0 {
		return nil, errors.New("image binder and declared protocol contracts are required")
	}
	contracts := append([]string(nil), declaredContracts...)
	sort.Strings(contracts)
	for index, contract := range contracts {
		if !validImageContract(contract) || index > 0 && contract == contracts[index-1] {
			return nil, errors.New("manifest contains an invalid or duplicate image protocol contract")
		}
	}
	handlers := make(map[string]imageProtocolOpenFunc, len(registrations))
	for _, registration := range registrations {
		if !validImageContract(registration.contract) || registration.open == nil || handlers[registration.contract] != nil {
			return nil, errors.New("image handler registration is invalid or duplicated")
		}
		handlers[registration.contract] = registration.open
	}
	if len(handlers) != len(contracts) {
		return nil, errors.New("manifest declarations and image handler registrations differ")
	}
	for _, contract := range contracts {
		if handlers[contract] == nil {
			return nil, errors.New("manifest declarations and image handler registrations differ")
		}
	}
	return &ImageDispatcher{binder: binder, contracts: contracts, handlers: handlers}, nil
}

func MustNewImageDispatcher(declaredContracts []string, binder ImageBinder, registrations ...ImageProtocolHandlerRegistration) *ImageDispatcher {
	dispatcher, err := NewImageDispatcher(declaredContracts, binder, registrations...)
	if err != nil {
		panic(err)
	}
	return dispatcher
}

func (d *ImageDispatcher) Bind(request BindRequest) (*BindSuccess, *DriverError) {
	if d == nil || d.binder == nil {
		return nil, internalError()
	}
	state, driverError := d.binder.BindImage(request)
	if driverError != nil {
		return nil, driverError
	}
	return &BindSuccess{BoundState: state, ImageCapabilities: &ImageCapabilities{ProtocolContracts: append([]string(nil), d.contracts...)}}, nil
}

func (d *ImageDispatcher) OpenImageAttempt(request ImageAttemptOpenRequest) (*ImageAttemptOpenResult, *DriverError) {
	if d == nil || request.Invocation == nil || request.Invocation.Request == nil {
		return nil, &DriverError{Code: ErrorInvalidInvocation}
	}
	open := d.handlers[request.Invocation.Request.ProtocolContract]
	if open == nil {
		return nil, &DriverError{Code: ErrorInvalidInvocation}
	}
	return open(request)
}
