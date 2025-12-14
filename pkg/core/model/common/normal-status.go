package common

type Status uint

const (
	NORMAL  = 1
	DISABLE = 2
)

type Flag int

const (
	TRUE  Flag = 1
	FALSE Flag = 2
)

func (f Flag) True() bool {
	return f == 1
}

func NewFlag(active bool) Flag {
	if active {
		return TRUE
	}
	return FALSE
}
