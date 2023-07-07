package plugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validTests(testDesc *string) []*string {
	tests := []*string{}
	prefix := "tag"
	for i := 1; i <= 5; i++ {
		test := fmt.Sprintf("[%s-%d] %s ID %d", prefix, i, *testDesc, i)
		tests = append(tests, &test)
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
			want:  "[total=5] [tag-1=1 (20.00%)] [tag-2=1 (20.00%)] [tag-3=1 (20.00%)] [tag-4=1 (20.00%)] [tag-5=1 (20.00%)]",
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
