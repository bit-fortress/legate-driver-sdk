package driver

import "testing"

func TestDispatcherRejectsManifestHandlerMismatch(t *testing.T) {
	binder := TextBinderFunc(func(BindRequest) ([]byte, *DriverError) { return nil, nil })
	handler := NewProtocolHandlerRegistration(ProtocolContractOpenAIChatCompletions20260718,
		func(TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError) { return nil, nil })
	if _, err := NewDispatcher([]string{ProtocolContractOpenAIChatCompletions20260718}, binder); err == nil {
		t.Fatal("missing handler was accepted")
	}
	if _, err := NewDispatcher([]string{ProtocolContractOpenAIResponses20260718}, binder, handler); err == nil {
		t.Fatal("handler for undeclared contract was accepted")
	}
	if _, err := NewDispatcher([]string{ProtocolContractOpenAIChatCompletions20260718}, binder, handler, handler); err == nil {
		t.Fatal("duplicate handler was accepted")
	}
}

func TestDispatcherPublishesRegisteredContracts(t *testing.T) {
	binder := TextBinderFunc(func(BindRequest) ([]byte, *DriverError) { return []byte("state"), nil })
	handler := NewProtocolHandlerRegistration(ProtocolContractOpenAIChatCompletions20260718,
		func(TextAttemptOpenRequest) (*TextAttemptOpenResult, *DriverError) { return nil, nil })
	dispatcher, err := NewDispatcher([]string{ProtocolContractOpenAIChatCompletions20260718}, binder, handler)
	if err != nil {
		t.Fatal(err)
	}
	bound, failure := dispatcher.Bind(BindRequest{})
	if failure != nil || string(bound.BoundState) != "state" || len(bound.TextCapabilities.ProtocolContracts) != 1 {
		t.Fatalf("bound = %#v failure = %#v", bound, failure)
	}
}
