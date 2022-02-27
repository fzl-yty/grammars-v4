package thrift

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"git.xiaojukeji.com/gulfstream/thriftpp/parser"
	"git.xiaojukeji.com/gulfstream/thriftpp/util"
)

type Type interface {
	Desc() string
}

type BaseType struct {
	Inner       string
	Annotations []AnnotationField
}

// Desc 返回描述信息。
func (t BaseType) Desc() string {
	return t.Inner
}

type RealBaseType string

// NewRealBaseType 构造函数
func NewRealBaseType(t string) RealBaseType {
	return RealBaseType(t)
}

// Desc 返回描述信息。
func (t RealBaseType) Desc() string {
	return string(t)
}

type UDFType string

// NewUDFType 构造函数
func NewUDFType(txt string) UDFType {
	return UDFType(txt)
}

// Desc 返回描述信息。
func (t UDFType) Desc() string {
	return string(t)
}

type ContainerType struct {
	Inner       Type
	Annotations []AnnotationField
	Comments
}

// Desc 返回描述信息。
func (t ContainerType) Desc() string {
	return t.Inner.Desc()
}

type ListType struct {
	Inner Type
}

// NewListType 构造函数
func NewListType(txt Type) ListType {
	return ListType{Inner: txt}
}

// Desc 返回描述信息。
func (t ListType) Desc() string {
	return t.Inner.Desc()
}

type SetType struct {
	Inner Type
}

// NewSetType 构造函数
func NewSetType(txt Type) SetType {
	return SetType{Inner: txt}
}

// Desc 返回描述信息。
func (t SetType) Desc() string {
	return t.Inner.Desc()
}

type MapType struct {
	Key   Type
	Value Type
}

// NewMapType 构造函数
func NewMapType(key, value Type) MapType {
	return MapType{
		Key:   key,
		Value: value,
	}
}

// Desc 返回描述信息。
func (t MapType) Desc() string {
	return "map[" + t.Key.Desc() + "]" + t.Value.Desc()
}

type StructType struct {
	Name        string
	SequenceNum int
	FieldType   []StructField
	Context     *parser.Struct_defContext
	Annotations []AnnotationField
	Comments
}

// Desc 返回描述信息。
func (structType StructType) Desc() string {
	return structType.Name
}

type TypedefType struct {
	Name         string
	SequenceNum  int
	OriginalType Type
	Context      *parser.TypedefContext
	Annotations  []AnnotationField
	Comments
}

// Desc 返回描述信息。
func (t TypedefType) Desc() string {
	return t.Name
}

type FunctionType struct {
	Name        string
	ServiceName string
	SequenceNum int
	Response    Type
	Requests    []StructField
	Annotations []AnnotationField
	Context     *parser.FunctionContext
	Comments
}

// Desc 返回描述信息。
func (fType FunctionType) Desc() string {
	return fType.Name
}

type EnumType struct {
	Name        string
	SequenceNum int
	Enumerate   []EnumField
	Context     *parser.Enum_ruleContext
	Annotations []AnnotationField
	Comments
}

// Desc 返回描述信息。
func (enumType EnumType) Desc() string {
	return enumType.Name
}

type EnumField struct {
	Key         string
	Value       int // according the idl, the value of enum field must be an integer
	Context     *parser.Enum_fieldContext
	Annotations []AnnotationField
	Comments
}

// nolint
type ConstType struct {
	Name    string
	Value   string
	Flag    string
	Type    string
	Context *parser.Const_ruleContext
	Comments
}

type Position struct {
	Start int
	End   int
}

type APIInfo struct {
	IDL         string
	Meta        map[string]Position
	Desc        APIDesc
	Annotations []AnnotationField
}

// APIDesc ...
type APIDesc struct {
	Path           string
	ReadTimeout    time.Duration
	ConnectTimeout time.Duration
	Method         string
	ContentType    string
}

// ServiceType ...
type ServiceType struct {
	Name        string
	Context     *parser.ServiceContext
	Annotations []AnnotationField
	Comments
	FunctionDefs map[string]FunctionType
}

// IncludeName ...
type IncludeName struct {
	Name    string
	Context *parser.IncludeContext
}

type Integer struct {
	Val  int
	Bits int
}

type Literal struct {
	Val string
}

type AnnotationField struct {
	Key   string
	Value util.Option
	Comments
}

type IDLService struct {
	ServiceName string
	Annotations []AnnotationField
	APIs        map[string]APIInfo
	MetaInfo    ServiceMetaInfo
	Comments
}

type StructField struct {
	FieldReq        FieldReq
	Name            string
	Context         *parser.FieldContext
	Type            Type
	FieldID         int
	Annotations     []AnnotationField
	DefaultValue    interface{}
	HasDefaultValue bool
	Comments
}

type FieldReq string

var (
	FieldReq_Required FieldReq = "required"
	FieldReq_Optional FieldReq = "optional"
)

type NameSpace struct {
	Scope   string
	Name    string
	Context *parser.NamespaceContext
}

type AnalysisResult struct {
	Typedefs     map[string]TypedefType
	StructDefs   map[string]StructType
	FunctionDefs map[string]FunctionType
	EnumDefs     map[string]EnumType
	ConstDefs    map[string]ConstType
	ServiceDef   ServiceType   // idl中定义的第一个service 适用于单idl
	ServiceDefs  []ServiceType // 多个service 适用于多idl
	IncludeNames []IncludeName
	Includes     map[string]AnalysisResult
	Namespaces   []NameSpace
}

type Comments struct {
	MultiLine  string
	SingleLine string
}

type ServiceMetaInfo struct {
	Version     string
	ServiceName string
	Protocol    string
	SLA         SLAInfo
}

type SLAInfo struct {
	ConnectTimeout int
	ReadTimeout    int
}

type KVPair struct {
	Key string
	Val string
}

type KeyWords struct {
	Keys []string `json:"keyWords"`
}

func (ans *AnalysisResult) CheckKeyWord(filePath string) error {

	var k = KeyWords{}
	// set 保留并去重关键字
	set := make(map[string]bool)

	if filePath != "" {
		key, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		json.Unmarshal(key, &k)
		for _, v := range k.Keys {
			set[v] = true
		}
	}

	for _, v := range keyWords {
		set[v] = true
	}

	for keyWord, _ := range set {
		if ans.ServiceDef.Name == keyWord {
			return fmt.Errorf("line:%d, ServiceDef include KeyWords: %s", ans.ServiceDef.Context.GetStart().GetStart(), keyWord)
		}

		for _, v := range ans.Typedefs {
			if v.Name == keyWord {
				return fmt.Errorf("line:%d, TypedefType include KeyWords: %s", v.Context.GetStart().GetStart(), keyWord)
			}
		}

		for _, v := range ans.StructDefs {
			if v.Name == keyWord {
				return fmt.Errorf("line:%d, StructDefs include KeyWords: %s", v.Context.GetStart().GetStart(), keyWord)
			}
		}

		for _, v := range ans.FunctionDefs {
			if v.Name == keyWord {
				return fmt.Errorf("line:%d, FunctionDefs include KeyWords: %s", v.Context.GetStart().GetStart(), keyWord)
			}
		}

		for _, v := range ans.EnumDefs {
			if v.Name == keyWord {
				return fmt.Errorf("line:%d, EnumDefs include KeyWords: %s", v.Context.GetStart().GetStart(), keyWord)
			}
		}
	}
	return nil
}
