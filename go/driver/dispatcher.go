package driver

import (
	"errors"
	"sort"
)

// TextBinder validates endpoint configuration and returns opaque bound state.
// Protocol capabilities are owned by Dispatcher and cannot be changed at bind time.
type TextBinder interface {
	BindText(BindRequest) ([]byte, *DriverError)
}

type TextBinderFunc func(BindRequest) ([]byte, *DriverError)

func (f TextBinderFunc) BindText(request BindRequest) ([]byte, *DriverError) {
	return f(request)
}

type protocolOpenFunc func(TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError)

// ProtocolHandlerRegistration is created by a versioned protocol package.
// Driver authors register typed handlers instead of switching on contract IDs.
type ProtocolHandlerRegistration struct {
	contract string
	open     protocolOpenFunc
}

func NewProtocolHandlerRegistration(contract string, open protocolOpenFunc) ProtocolHandlerRegistration {
	return ProtocolHandlerRegistration{contract: contract, open: open}
}

// Dispatcher is a Driver that routes exact contracts to typed protocol handlers.
type Dispatcher struct {
	binder    TextBinder
	contracts []string
	handlers  map[string]protocolOpenFunc
}

// NewDispatcher rejects any mismatch between manifest declarations and typed
// handler registrations. Declared contracts are normalized into stable order.
func NewDispatcher(declaredContracts []string, binder TextBinder, registrations ...ProtocolHandlerRegistration) (*Dispatcher, error) {
	if binder == nil || len(declaredContracts) == 0 {
		return nil, errors.New("text binder and declared protocol contracts are required")
	}
	contracts := append([]string(nil), declaredContracts...)
	sort.Strings(contracts)
	for index, contract := range contracts {
		if !validContract(contract) || index > 0 && contract == contracts[index-1] {
			return nil, errors.New("manifest contains an invalid or duplicate protocol contract")
		}
	}
	handlers := make(map[string]protocolOpenFunc, len(registrations))
	for _, registration := range registrations {
		if !validContract(registration.contract) || registration.open == nil {
			return nil, errors.New("handler registration is invalid")
		}
		if _, exists := handlers[registration.contract]; exists {
			return nil, errors.New("protocol contract has multiple handlers")
		}
		handlers[registration.contract] = registration.open
	}
	if len(handlers) != len(contracts) {
		return nil, errors.New("manifest declarations and handler registrations differ")
	}
	for _, contract := range contracts {
		if handlers[contract] == nil {
			return nil, errors.New("manifest declarations and handler registrations differ")
		}
	}
	return &Dispatcher{binder: binder, contracts: contracts, handlers: handlers}, nil
}

func MustNewDispatcher(declaredContracts []string, binder TextBinder, registrations ...ProtocolHandlerRegistration) *Dispatcher {
	dispatcher, err := NewDispatcher(declaredContracts, binder, registrations...)
	if err != nil {
		panic(err)
	}
	return dispatcher
}

func (d *Dispatcher) ProtocolContracts() []string {
	if d == nil {
		return nil
	}
	return append([]string(nil), d.contracts...)
}

func (d *Dispatcher) Bind(request BindRequest) (*BindSuccess, *DriverError) {
	if d == nil || d.binder == nil {
		return nil, internalError()
	}
	state, driverError := d.binder.BindText(request)
	if driverError != nil {
		return nil, driverError
	}
	return &BindSuccess{
		BoundState: state,
		TextCapabilities: &TextCapabilities{
			ProtocolContracts: append([]string(nil), d.contracts...),
		},
	}, nil
}

func (d *Dispatcher) OpenTextAttempt(request TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError) {
	if d == nil || request.Invocation == nil || request.Invocation.Request == nil {
		return nil, &DriverError{Code: ErrorInvalidInvocation}
	}
	open := d.handlers[request.Invocation.Request.ProtocolContract]
	if open == nil {
		return nil, &DriverError{Code: ErrorInvalidInvocation}
	}
	return open(request)
}
