package mint

import (
	"reflect"
	"testing"
)

func TestNewDimension(t *testing.T) {
	type args struct {
		key   string
		value string
	}
	tests := []struct {
		name string
		args args
		want Dimension
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDimension(tt.args.key, tt.args.value); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDimension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDimension_toString(t *testing.T) {
	type fields struct {
		key   string
		value string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dimension{
				key:   tt.fields.key,
				value: tt.fields.value,
			}
			if got := d.toString(); got != tt.want {
				t.Errorf("Dimension.toString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDescriptor(t *testing.T) {
	type args struct {
		name       string
		prefix     string
		dimensions []Dimension
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Basic descriptor",
			args: args{name: "metric_name"},
			want: "metric_name",
		},
		{
			name: "Prefixed descriptor",
			args: args{name: "metric_name", prefix: "prefix"},
			want: "prefix.metric_name",
		},
		{
			name: "Descriptor with dimension",
			args: args{name: "metric_name", dimensions: []Dimension{{key: "dim", value: "value"}}},
			want: "metric_name,dim=\"value\"",
		},
		{
			name: "Descriptor with prefix and dimension",
			args: args{name: "metric_name", prefix: "prefix", dimensions: []Dimension{{key: "dim", value: "value"}}},
			want: "prefix.metric_name,dim=\"value\"",
		},
		{
			name: "Invalid prefix",
			args: args{name: "metric_name", prefix: "."},
			want: "",
		},
		{
			name: "Invalid leading characters",
			args: args{name: "metric_name", prefix: "!prefix."},
			want: "prefix.metric_name",
		},
		{
			name: "just.a.normal.key",
			args: args{name: "just.a.normal.key"},
			want: "just.a.normal.key",
		},
		{
			name: "Case",
			args: args{name: "Case"},
			want: "Case",
		},
		{
			name: "~0something",
			args: args{name: "~0something"},
			want: "something",
		},
		{
			name: "some~thing",
			args: args{name: "some~thing"},
			want: "some_thing",
		},
		{
			name: "some~ä#thing",
			args: args{name: "some~ä#thing"},
			want: "some_thing",
		},
		{
			name: "a..b",
			args: args{name: "a..b"},
			want: "a.b",
		},
		{
			name: "a.....b",
			args: args{name: "a.....b"},
			want: "a.b",
		},
		{
			name: "asd",
			args: args{name: "asd"},
			want: "asd",
		},
		{
			name: ".",
			args: args{name: "."},
			want: "",
		},
		{
			name: ".a",
			args: args{name: ".a"},
			want: "",
		},
		{
			name: "a.",
			args: args{name: "a."},
			want: "a",
		},
		{
			name: ".a.",
			args: args{name: ".a."},
			want: "",
		},
		{
			name: "_a",
			args: args{name: "_a"},
			want: "a",
		},
		{
			name: "a_",
			args: args{name: "a_"},
			want: "a_",
		},
		{
			name: "_a_",
			args: args{name: "_a_"},
			want: "a_",
		},
		{
			name: ".a_",
			args: args{name: ".a_"},
			want: "",
		},
		{
			name: "_a.",
			args: args{name: "_a."},
			want: "a",
		},
		{
			name: "._._a_._._",
			args: args{name: "._._a_._._"},
			want: "",
		},
		{
			name: "test..empty.test",
			args: args{name: "test..empty.test"},
			want: "test.empty.test",
		},
		{
			name: "a,,,b  c=d\\e\\ =,f",
			args: args{name: "a,,,b  c=d\\e\\ =,f"},
			want: "a_b_c_d_e_f",
		},
		{
			name: "a!b\"c#d$e%f&g'h(i)j*k+l,m-n.o/p:q;r<s=t>u?v@w[x]y\\z^0 1_2;3{4|5}6~7",
			args: args{name: "a!b\"c#d$e%f&g'h(i)j*k+l,m-n.o/p:q;r<s=t>u?v@w[x]y\\z^0 1_2;3{4|5}6~7"},
			want: "a_b_c_d_e_f_g_h_i_j_k_l_m-n.o_p_q_r_s_t_u_v_w_x_y_z_0_1_2_3_4_5_6_7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeDescriptor(tt.args.name, tt.args.prefix, tt.args.dimensions); got != tt.want {
				t.Errorf("SerializeDescriptor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeDimensions(t *testing.T) {
	type args struct {
		dimensions []Dimension
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeDimensions(tt.args.dimensions); got != tt.want {
				t.Errorf("serializeDimensions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_joinPrefix(t *testing.T) {
	type args struct {
		name   string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinPrefix(tt.args.name, tt.args.prefix); got != tt.want {
				t.Errorf("joinPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeRecord(t *testing.T) {
	type args struct {
		descriptor string
		valueLine  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeRecord(tt.args.descriptor, tt.args.valueLine); got != tt.want {
				t.Errorf("SerializeRecord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeIntSummaryValue(t *testing.T) {
	type args struct {
		min   int64
		max   int64
		sum   int64
		count int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeIntSummaryValue(tt.args.min, tt.args.max, tt.args.sum, tt.args.count); got != tt.want {
				t.Errorf("SerializeIntSummaryValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleSummaryValue(t *testing.T) {
	type args struct {
		min   float64
		max   float64
		sum   float64
		count int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeDoubleSummaryValue(tt.args.min, tt.args.max, tt.args.sum, tt.args.count); got != tt.want {
				t.Errorf("SerializeDoubleSummaryValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeIntCountValue(t *testing.T) {
	type args struct {
		value int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeIntCountValue(tt.args.value); got != tt.want {
				t.Errorf("SerializeIntCountValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSerializeDoubleCountValue(t *testing.T) {
	type args struct {
		value float64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SerializeDoubleCountValue(tt.args.value); got != tt.want {
				t.Errorf("SerializeDoubleCountValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serializeFloat64(t *testing.T) {
	type args struct {
		n float64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serializeFloat64(tt.args.n); got != tt.want {
				t.Errorf("serializeFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}
