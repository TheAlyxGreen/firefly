package firefly

/*
	Bsky requires an insane amount of nil-checking, so these functions take a pointer to a value and either return the
	value or a zero value if the pointer is nil
*/

func safeStringToValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func safeI64ToValue(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func safeI64ToInt(i *int64) int {
	if i == nil {
		return 0
	} else {
		return int(*i)
	}
}
