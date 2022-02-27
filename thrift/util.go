package thrift

type Option struct {
	value  interface{}
	isSome bool
}

func Some(value interface{}) Option {
	return Option{
		value:  value,
		isSome: true,
	}
}

func None() Option {
	return Option{
		isSome: false,
	}
}

func (o Option) IsNull() bool {
	return !o.IsSome()
}

func (o Option) IsSome() bool {
	return o.isSome
}

// Value ...
func (o Option) Value() interface{} {
	return o.value
}

func (o Option) UnwrapOr(value interface{}) interface{} {
	if o.IsSome() {
		return o.value
	}
	return value
}

func (o Option) UnwrapOrString(value string) string {
	if o.IsSome() {
		v, ok := o.value.(string)
		if ok {
			return v
		}
	}
	return value
}
