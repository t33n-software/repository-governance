package canonical

import "testing"

// FuzzDecodeBindings is the boundary-fuzz lane for the binding manifest: the
// decoder must never panic and must fail closed on any malformed input.
func FuzzDecodeBindings(f *testing.F) {
	f.Add([]byte(validBindingsJSON()))
	f.Add([]byte(`{"schemaVersion":1}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeBindings(data)
	})
}
