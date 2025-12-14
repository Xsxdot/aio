package utils

import "testing"

func TestGCJ02ToBD09(t *testing.T) {
	type args struct {
		lon float64
		lat float64
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 float64
	}{
		{
			name: "",
			args: args{
				lon: 31.40778628,
				lat: 121.34895713,
			},
			want:  0,
			want1: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GCJ02ToBD09(tt.args.lon, tt.args.lat)
			if got != tt.want {
				t.Errorf("GCJ02ToBD09() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GCJ02ToBD09() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
