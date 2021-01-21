package errorutil

import (
	"fmt"
	"testing"

	liberr "github.com/konveyor/controller/pkg/error"
)

func TestUnwrap(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test when Unwrap is implemented",
			args:    args{err: liberr.Wrap(fmt.Errorf("wrap is implemented"))},
			wantErr: true,
		},
		{
			name:    "test when Unwrap is not implemented",
			args:    args{err: fmt.Errorf("wrap is not implemented")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Unwrap(tt.args.err); (err != nil) != tt.wantErr {
				t.Errorf("Unwrap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
