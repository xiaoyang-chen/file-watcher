//go:build windows
// +build windows

package watcher

import (
	"os"
	"testing"
)

func Test_isHiddenFileEx(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name         string
		args         args
		wantIsHidden bool
		wantErr      bool
	}{
		{
			name: "TestIsHiddenFileExReturnsPathError",
			args: args{
				path: "./qqdkqdsdmlqdsd.nop",
			},
			wantIsHidden: false,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsHidden, err := isHiddenFileEx(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("isHiddenFileEx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIsHidden != tt.wantIsHidden {
				t.Errorf("isHiddenFileEx() = %v, want %v", gotIsHidden, tt.wantIsHidden)
			}
			if tt.name == "TestIsHiddenFileExReturnsPathError" {
				if _, ok := err.(*os.PathError); !ok {
					t.Errorf("isHiddenFileEx() error = %v, wantErr %v, err is not a os.PathError", err, tt.wantErr)
				}
			}
		})
	}
}
