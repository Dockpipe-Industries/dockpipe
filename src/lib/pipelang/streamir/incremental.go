package streamir

// V3 borrows host-owned sessions and spans; source never owns pointers or handles.
func ManifestProfile(profile string) string {
	if profile == IncrementalProfile {
		return IncrementalProfile
	}
	return Profile
}
func NativeSignature(profile string) ([]Type, Type) {
	if profile == IncrementalProfile {
		return []Type{Session, InputBuffer, OutputBuffer, Bool}, Step
	}
	return []Type{ReadStream, WriteStream, Int}, Result
}
func ValueTypeFor(profile string, t Type) bool {
	return ValueType(t) && (t != Step || profile == IncrementalProfile)
}
func ParameterTypeFor(profile string, t Type) bool {
	if t == Session || t == InputBuffer || t == OutputBuffer || t == Step {
		return profile == IncrementalProfile
	}
	return ParameterType(t)
}
func ResultMemberType(t Type, name string) Type {
	if t != Result && t != Step {
		return ""
	}
	if t == Step {
		switch name {
		case "needInput", "needOutput", "done":
			return Bool
		case "inputConsumed", "outputWritten":
			return Uint64
		}
	}
	return MemberType(name)
}

type Progress uint32

const (
	NoProgress Progress = iota
	NeedInput
	NeedOutput
	Done
)
