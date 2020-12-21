package homie

// Validates that an ID conforms to the Homie standard.

func validate(inputId string, attr bool) string {
	bytes := []byte(inputId)

	if bytes[0] == '_' {
		panic("Identifier may not begin with '_'")
	}

	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + 'a' - 'A'
		} else if (b < 'a' || b > 'z') &&
			(b < '0' || b > '9') &&
			(i != 0 || b != '$' || !attr) &&
			b != '_' {
			panic("Invalid character in identifier")
		}
	}

	return string(bytes)
}
