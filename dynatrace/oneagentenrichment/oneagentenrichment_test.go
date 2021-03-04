package oneagentenrichment

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/dynatrace-oss/opentelemetry-metric-go/mint"
	"go.uber.org/zap"
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

func Test_readOneAgentMetadata(t *testing.T) {
	type args struct {
		indirectionBasename string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "valid case",
			args: args{indirectionBasename: "testdata/indirection"},
			want: []string{"key1=value1", "key2=value2", "key3=value3"},
		},
		{
			name: "metadata file empty",
			args: args{indirectionBasename: "testdata/indirection_target_empty"},
			want: []string{},
		},
		{
			name:    "indirection file empty",
			args:    args{indirectionBasename: "testdata/indirection_empty"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "indirection file does not exist",
			args:    args{indirectionBasename: "testdata/indirection_file_that_does_not_exist"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "indirection target does not exist",
			args:    args{indirectionBasename: "testdata/indirection_target_nonexistent"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readOneAgentMetadata(tt.args.indirectionBasename)
			if (err != nil) != tt.wantErr {
				t.Errorf("readOneAgentMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readOneAgentMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOneAgentMetadataEnricher_parseOneAgentMetadata(t *testing.T) {
	log, _ := zap.NewProduction()
	defer log.Sync()
	type fields struct {
		logger *zap.Logger
	}
	type args struct {
		lines []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []mint.Dimension
	}{
		{
			name:   "valid case",
			fields: fields{logger: log},
			args:   args{[]string{"key1=value1", "key2=value2", "key3=value3"}},
			want: []mint.Dimension{
				mint.NewDimension("key1", "value1"),
				mint.NewDimension("key2", "value2"),
				mint.NewDimension("key3", "value3"),
			},
		},
		{
			name:   "pass empty list",
			fields: fields{logger: log},
			args:   args{[]string{}},
			want:   []mint.Dimension{},
		},
		{
			name:   "pass invalid strings",
			fields: fields{logger: log},
			args: args{[]string{
				"=0x5c14d9a68d569861",
				"otherKey=",
				"",
				"=",
				"===",
			}},
			want: []mint.Dimension{},
		},
		{
			name:   "pass mixed strings",
			fields: fields{logger: log},
			args: args{[]string{
				"invalid1",
				"key1=value1",
				"=invalid",
				"key2=value2",
				"===",
			}},
			want: []mint.Dimension{
				mint.NewDimension("key1", "value1"),
				mint.NewDimension("key2", "value2"),
			},
		},
		{
			name:   "valid tailing equal signs",
			fields: fields{logger: log},
			args:   args{[]string{"key1=value1=="}},
			want:   []mint.Dimension{mint.NewDimension("key1", "value1==")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := OneAgentMetadataEnricher{
				logger: tt.fields.logger,
			}
			if got := o.parseOneAgentMetadata(tt.args.lines); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OneAgentMetadataEnricher.parseOneAgentMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
