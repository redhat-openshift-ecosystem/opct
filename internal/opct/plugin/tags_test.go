package plugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validTests(testDesc *string) []*string {
	tests := []*string{}
	prefix := "tag"
	max := 5

	for i := 1; i <= max; i++ {
		for x := (max - i); x >= 0; x-- {
			test := fmt.Sprintf("[%s-%d] %s ID %d", prefix, i, *testDesc, i)
			tests = append(tests, &test)
		}
	}
	return tests
}

func TestShowSorted(t *testing.T) {
	desc := "TestShowSorted"
	cases := []struct {
		name  string
		tests []*string
		want  string
	}{
		{
			name:  "empty",
			tests: validTests(&desc),
			want:  "[total=15] [tag-1=5 (33.33%)] [tag-2=4 (26.67%)] [tag-3=3 (20.00%)] [tag-4=2 (13.33%)] [tag-5=1 (6.67%)]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// fmt.Printf("%v\n", tc.tests)
			testTags := NewTestTags(tc.tests)
			msg := testTags.ShowSorted()
			assert.Equal(t, tc.want, msg, "unexpected ,essage")
		})
	}
}
