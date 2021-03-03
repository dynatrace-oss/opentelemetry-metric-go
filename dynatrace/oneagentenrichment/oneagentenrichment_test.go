package oneagentenrichment

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func Test_readIndirectionFile(t *testing.T) {
	type args struct {
		reader io.Reader
		prefix string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "valid case",
			args: args{reader: strings.NewReader("prefix_metadata_file.txt"), prefix: "prefix"},
			want: "prefix_metadata_file.txt",
		},
		{
			name: "empty file",
			args: args{reader: strings.NewReader(""), prefix: "prefix"},
			want: "",
		},
		{
			name: "empty prefix",
			args: args{reader: strings.NewReader("metadata_file.txt"), prefix: ""},
			want: "metadata_file.txt",
		},
		{
			name: "missing prefix in file",
			args: args{reader: strings.NewReader("metadata_file.txt"), prefix: "prefix"},
			want: "",
		},
		{
			name: "multiline file",
			args: args{reader: strings.NewReader("this\nfile\ncontains\nprefix_metadata_file.txt\nmultiple\nlines"), prefix: "prefix"},
			want: "prefix_metadata_file.txt",
		},
		{
			name: "whitespace",
			args: args{reader: strings.NewReader("\t \t prefix_metadata_file.txt\t \t\n"), prefix: "prefix"},
			want: "prefix_metadata_file.txt",
		},
		{
			name:    "pass nil reader",
			args:    args{reader: nil, prefix: "prefix"},
			wantErr: true,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readIndirectionFile(tt.args.reader, tt.args.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("readIndirectionFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("readIndirectionFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readMetadataFile(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "valid case",
			args: args{reader: strings.NewReader("key1=value1\nkey2=value2")},
			want: []string{"key1=value1", "key2=value2"},
		},
		{
			name: "metadata file empty",
			args: args{reader: strings.NewReader("")},
			want: []string{},
		},
		{
			name: "ignore whitespace",
			args: args{reader: strings.NewReader("\t \tkey1=value1\t \t\n\t \tkey2=value2\t \t")},
			want: []string{"key1=value1", "key2=value2"},
		},
		{
			name: "empty lines ignored",
			args: args{reader: strings.NewReader("\n\t \t\n")},
			want: []string{},
		},
		{
			name:    "pass nil reader",
			args:    args{reader: nil},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readMetadataFile(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("readMetadataFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readMetadataFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
