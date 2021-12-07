package settings

import (
	"reflect"
	"testing"
)

func Test_parseIntListFromString(t *testing.T) {
	tests := []struct {
		name    string
		strList string
		want    []int64
		wantErr bool
	}{
		{
			name:    "given a list of valid integer strings, must return list of integers, must not return error",
			strList: "1000690000,1000670000,1000820000",
			want: []int64{
				1000690000,
				1000670000,
				1000820000,
			},
			wantErr: false,
		},
		{
			name:    "given a list of valid integer strings, must return list of integers, must not return error",
			strList: "1000690000",
			want: []int64{
				1000690000,
			},
			wantErr: false,
		},
		{
			name:    "given a list of empty strings, must return empty list, must not return error",
			strList: ",,",
			want:    []int64{},
			wantErr: false,
		},
		{
			name:    "given a list of empty strings, must return empty list, must not return error",
			strList: "",
			want:    []int64{},
			wantErr: false,
		},
		{
			name:    "given a list of invalid integer strings, must return empty list, must return error",
			strList: "s,d,",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIntListFromString(tt.strList)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIntListFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIntListFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}
