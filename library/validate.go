package homie

// Validates that an ID conforms to the Homie standard.

func validate(inputId string, attr bool) string {
	if len(inputId) < 1 {
		panic("Invalid use of null identifier")
	}

	bytes := []byte(inputId)

	if bytes[0] == '-' {
		panic("Identifier may not begin with '-'")
	}

	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + 'a' - 'A'
		} else if (b < 'a' || b > 'z') &&
			(b < '0' || b > '9') &&
			(i != 0 || b != '$' || !attr) &&
			b != '-' {
			panic("Invalid character in identifier")
		}
	}

	return string(bytes)
}
