package archive

import (
	"reflect"
	"testing"

	"k8s.io/utils/ptr"
)

func TestMergeErrorCounters(t *testing.T) {
	type args struct {
		ec1 *ErrorCounter
		ec2 *ErrorCounter
	}
	tests := []struct {
		name string
		args args
		want *ErrorCounter
	}{
		{
			name: "merge both",
			args: args{
				ec1: &ErrorCounter{"error": 1, `level":"fatal"`: 10},
				ec2: &ErrorCounter{"error": 1, `level":"fatal"`: 0},
			},
			want: &ErrorCounter{"error": 2, `level":"fatal"`: 10},
		},
		{
			name: "merge both",
			args: args{
				ec1: &ErrorCounter{"error": 20000},
				ec2: &ErrorCounter{"error": 1, `level":"fatal"`: 0},
			},
			want: &ErrorCounter{"error": 20001, `level":"fatal"`: 0},
		},
		{
			name: "both null",
			args: args{
				ec1: nil,
				ec2: nil,
			},
			want: &ErrorCounter{},
		},
		{
			name: "ec1 null",
			args: args{
				ec1: nil,
				ec2: &ErrorCounter{"error": 1},
			},
			want: &ErrorCounter{"error": 1},
		},
		{
			name: "ec2 null",
			args: args{
				ec1: &ErrorCounter{"error": 1},
				ec2: nil,
			},
			want: &ErrorCounter{"error": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeErrorCounters(tt.args.ec1, tt.args.ec2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeErrorCounters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewErrorCounter(t *testing.T) {
	type args struct {
		buf     *string
		pattern []string
	}
	tests := []struct {
		name string
		args args
		want ErrorCounter
	}{
		{
			name: "parse counters",
			args: args{
				buf: ptr.To(`this buffer has one error,
					and another 'ERROR:', also crashs with 'panic.go:12:'.
					Some messages of Failed to push image`),
				pattern: CommonErrorPatterns,
			},
			want: ErrorCounter{
				`'ERROR:'`: 1, `Failed`: 1, `Failed to push image`: 1,
				`error`: 1, `panic(\.go)?:`: 1, `total`: 5,
			},
		},
		{
			name: "no counters",
			args: args{
				buf:     ptr.To(`this buffer has nothing to parse`),
				pattern: CommonErrorPatterns,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewErrorCounter(tt.args.buf, tt.args.pattern); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewErrorCounter() = %v, want %v", got, tt.want)
			}
		})
	}
}
