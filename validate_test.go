package homie

// test the validate routine

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidate_0(t *testing.T) {
	// test a valid id
	i := "now_is_the_time"
	s := validate(i, false)
	if i != s {
		t.Errorf("validate(%s) yields %s", i, s)
	}
}

func TestValidate_1(t *testing.T) {
	// test a valid id
	i := "now_Is_The_Time"
	s := validate(i, false)
	if strings.ToLower(i) != s {
		t.Errorf("validate(%s) yields %s", i, s)
	}
}

func TestValidate_2(t *testing.T) {
	// test an invalid id
	i := "now_Is_The_$_Time"

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("recovered %s\n", r)
			fmt.Printf("input was %s\n", i)
		}
	}()

	validate(i, true)
	t.Fatalf("id %s did not panic", i)
}
