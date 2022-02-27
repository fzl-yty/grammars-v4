package thrift

import (
	"bytes"
	"fmt"
	"git.xiaojukeji.com/gulfstream/thriftpp/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var _baseTypeConvert = map[string]string{
	"bool":   "bool",
	"byte":   "uint32",
	"i8":     "int32",
	"i16":    "int32",
	"i32":    "int32",
	"i64":    "int64",
	"double": "double",
	"string": "string",
	"binary": "bytes",
}

const (
	_MessageKW  = "message"
	_EnumKW     = "enum"
	_RepeatedKW = "repeated"
	_RPCKW      = "rpc"
)

type CodeGenOpt struct {
	IsProto3WithOptional bool
}

type sortEnumField []EnumField

func (fields sortEnumField) Len() int {
	return len(fields)
}

func (fields sortEnumField) Swap(i, j int) {
	fields[i], fields[j] = fields[j], fields[i]
}

func (fields sortEnumField) Less(i, j int) bool {
	return fields[i].Value < fields[j].Value
}

func (enumType EnumType) Gen(buf *IdentBuffer, _ map[string]TypedefType, _ CodeGenOpt) error {
	sort.Sort(sortEnumField(enumType.Enumerate))

	if enumType.Comments.MultiLine != "" {
		buf.WriteLine(multiLineFormat(enumType.Comments.MultiLine))
	} else if enumType.Comments.SingleLine != "" {
		buf.WriteLine(singleLineFormat(enumType.Comments.SingleLine))
	}

	buf.WriteLine(_EnumKW + " " + enumType.Name + " {")
	n := len(enumType.Enumerate)
	for i := 0; i < n; i++ {
		e := enumType.Enumerate[i]
		buf := buf.Derive(1)
		// buf.WriteString(e.MultiLine)
		buf.WriteString(e.Key)
		buf.WriteByte('=')
		v := e.Value
		if v == 0 {
			v = i
		}
		buf.WriteString(strconv.Itoa(v))
		buf.WriteByte(';')
		if e.SingleLine != "" {
			buf.WriteString(singleLineFormat(e.SingleLine))
		} else if desc := GetAnnotationString(e.Annotations, "desc"); desc != "" {
			comment, err := strconv.Unquote(desc)
			if err == nil {
				buf.WriteString(" //")
				buf.WriteString(strings.TrimSpace(comment))
			}
		} else if e.MultiLine != "" {
			buf.WriteString(" ")
			buf.WriteString(multiLineFormat((e.MultiLine)))
		}
		buf.NewLine(1)
	}
	buf.WriteLine("}")
	return nil
}

func (structType StructType) Gen(buf *IdentBuffer, typedefs map[string]TypedefType, opt CodeGenOpt) error {

	if structType.MultiLine != "" {
		buf.WriteLine(multiLineFormat(structType.MultiLine))
	} else if structType.SingleLine != "" {
		buf.WriteLine(singleLineFormat(structType.SingleLine))
	}

	buf.WriteLine(_MessageKW + " " + structType.Name + " {")
	for _, field := range structType.FieldType {
		buf := buf.Derive(1)
		// if field.MultiLine != "" {
		// 	buf.WriteLine(field.MultiLine)
		// }
		typeName := expandType(field.Type, typedefs)
		if typeName == "" {
			return fmt.Errorf("fail to expand field %s of %s", field.Name, structType.Name)
		}

		if opt.IsProto3WithOptional && field.FieldReq == FieldReq_Optional &&
			!strings.HasPrefix(strings.TrimSpace(typeName), _RepeatedKW) && !strings.HasPrefix(strings.TrimSpace(typeName), "map<") {
			buf.WriteString("optional")
			buf.WriteByte(' ')
		}

		buf.WriteString(typeName)
		buf.WriteByte(' ')
		buf.WriteString(field.Name)
		buf.WriteString(" = ")
		buf.WriteString(strconv.Itoa(field.FieldID))

		if err := checkValidOption(field.Annotations); err != nil {
			return err
		}

		hasOption := false
		if n := len(field.Annotations); n > 0 {
			hasOption = GetAnnotation(field.Annotations, "desc").IsNull() || n > 1
		}

		if hasOption {
			buf.WriteString(" [")
		}

		if jsonName := GetAnnotationString(field.Annotations, "json"); jsonName != "" {
			buf.WriteString("json_name = ")
			buf.WriteString(jsonName)
		}

		if hasOption {
			buf.WriteString("]")
		}

		buf.WriteString(";")
		if field.SingleLine != "" {
			buf.WriteString(singleLineFormat(field.SingleLine))
		} else if desc := GetAnnotationString(field.Annotations, "desc"); desc != "" {
			comment, err := strconv.Unquote(desc)
			if err == nil {
				buf.WriteString(" //")
				buf.WriteString(strings.TrimSpace(comment))
			}
		} else if field.MultiLine != "" {
			buf.WriteString(" ")
			buf.WriteString(multiLineFormat(field.MultiLine))
		}
		buf.NewLine(1)
	}
	buf.WriteLine("}")
	return nil
}

func checkValidOption(annotations []AnnotationField) error {
	var err error
	for _, a := range annotations {
		if a.Key != "desc" && a.Key != "json" {
			if err == nil {
				err = fmt.Errorf("invalid field annotation %q", a.Key)
			} else {
				err = fmt.Errorf("invalid field annotation %q, %w", a.Key, err)
			}
		}
	}
	return err
}

func expandType(fieldType interface{}, typedefs map[string]TypedefType) string {
	switch t := fieldType.(type) {
	case UDFType:
		tdf, ok := typedefs[string(t)]
		if !ok {
			return string(t)
		}
		return expandType(tdf.OriginalType, typedefs)
	case ContainerType:
		return expandType(t.Inner, typedefs)
	case SetType:
		return expandType(ListType{Inner: t.Inner}, typedefs)
	case ListType:
		v := expandType(t.Inner, typedefs)
		if v != "" {
			return _RepeatedKW + " " + v
		}
	case MapType:
		if k := expandType(t.Key, typedefs); k != "" {
			if v := expandType(t.Value, typedefs); v != "" {
				return fmt.Sprintf("map<%s, %s>", k, v)
			}
		}
	case BaseType:
		return expandType(t.Inner, nil)
	case RealBaseType:
		if v, ok := _baseTypeConvert[string(t)]; ok {
			return v
		}
	case string:
		if v, ok := _baseTypeConvert[t]; ok {
			return v
		}
	}
	return ""
}

func (fType FunctionType) Gen(buf *IdentBuffer, typedefs map[string]TypedefType, _ CodeGenOpt) error {

	if fType.MultiLine != "" {
		buf.WriteLine(multiLineFormat(fType.MultiLine))
	} else if fType.SingleLine != "" {
		buf.WriteLine(singleLineFormat(fType.SingleLine))
	}

	respType := expandType(fType.Response, typedefs)
	if respType == "" {
		return fmt.Errorf("fail to expand response type %v of function %s", fType.Response, fType.Name)
	}
	if len(fType.Requests) != 1 {
		return fmt.Errorf("grpc requires exact one input, function %s has %d", fType.Name, len(fType.Requests))
	}
	reqType := expandType(fType.Requests[0].Type, typedefs)
	if reqType == "" {
		return fmt.Errorf("fail to expand request param %v of function %s", fType.Requests[0], fType.Name)
	}
	buf.WriteLine(fmt.Sprintf("rpc %s ( %s ) returns ( %s ) {", fType.Name, reqType, respType))
	fType.httpOption(buf.Derive(1))
	buf.WriteString("}")
	buf.WriteLine(singleLineFormat(fType.SingleLine))
	return nil
}

func (fType FunctionType) httpOption(buf *IdentBuffer) {
	path := strings.Trim(GetAnnotationString(fType.Annotations, "path"), `"`)
	method := strings.Trim(GetAnnotationString(fType.Annotations, "httpMethod"), `"`)
	if path != "" && method != "" {
		buf.WriteLine("option (google.api.http) = {")
		ibuf := buf.Derive(1)
		lowMethod := strings.ToLower(method)
		ibuf.WriteLine(fmt.Sprintf("%s: \"%s\"", lowMethod, path))
		if lowMethod == "post" || lowMethod == "put" {
			ibuf.WriteLine("body: \"*\"")
		}
		buf.WriteLine("};")
	}

	httpOptions := getHTTPOption(fType.Annotations)
	if len(httpOptions) > 0 {
		buf.WriteLine("option (dirpc.method_opt) = {")
		ibuf := buf.Derive(1)
		for _, opt := range httpOptions {
			ibuf.WriteLine(fmt.Sprintf("%s: %s", opt.Key, opt.Val))
		}
		buf.WriteLine("};")
	}

}

func (ast AnalysisResult) CodeGen(currentName, lang string, mainPackage bool, opt CodeGenOpt) (map[string]string, error) {
	buf := NewIdentWriter(0)
	buf.WriteLine(`syntax = "proto3";`)
	pkgName := currentName
	for _, ns := range ast.Namespaces {
		if ns.Scope == lang {
			pkgName = ns.Name
			break
		}
	}
	buf.WriteLine(fmt.Sprintf("package %s;", pkgName))
	buf.NewLine(1)
	buf.WriteLine("// auto generated by thriftpp, contact chibi@didiglobal.com for reporting bug")
	if mainPackage {
		buf.WriteLine("import \"google/api/annotations.proto\";")
		// buf.WriteLine("import \"protoc-gen-swagger/options/annotations.proto\";")
		buf.WriteLine("import \"dirpc/dirpc.proto\";")

	}

	for _, includeFile := range ast.IncludeNames {

		fileName := filepath.Base(includeFile.Name)
		idx := strings.IndexByte(fileName, '.')
		if idx != -1 {
			fileName = fileName[:idx]
		}

		buf.WriteLine(fmt.Sprintf(`import "%s.proto";`, fileName))
	}
	buf.NewLine(1)

	if lang == "php" {
		buf.WriteLine(fmt.Sprintf("option php_namespace = \"%s\";", strings.ReplaceAll(pkgName, ".", "\\\\")))
		buf.WriteLine(fmt.Sprintf("option php_metadata_namespace = \"%s\\\\GPBMetadata\";", strings.ReplaceAll(pkgName, ".", "\\\\")))
	}

	// if mainPackage {
	// 	buf.WriteLine("option (grpc.gateway.protoc_gen_swagger.options.openapiv2_swagger) = {")
	// 	ibuf := buf.Derive(1)
	// 	ibuf.WriteLine("host: \"127.0.0.1:8991\"")
	// 	ibuf = nil
	// 	buf.WriteLine("};")
	// }
	buf.NewLine(1)
	for _, edf := range sortEnumDefs(ast.EnumDefs) {
		err := edf.Gen(buf, nil, opt)
		if err != nil {
			return nil, err
		}
		buf.NewLine(1)
	}
	for _, stdef := range sortStructDefs(ast.StructDefs) {
		err := stdef.Gen(buf, ast.Typedefs, opt)
		if err != nil {
			return nil, err
		}
		buf.NewLine(1)
	}

	if ast.ServiceDef.Name != "" {
		buf.WriteLine(multiLineFormat(ast.ServiceDef.MultiLine))
		buf.WriteLine(fmt.Sprintf("service %s {", ast.ServiceDef.Name))
		for _, fdef := range SortFunctionDefs(ast.FunctionDefs) {
			// 判断是否是该service下的function
			if fdef.ServiceName != ast.ServiceDef.Name {
				continue
			}
			err := fdef.Gen(buf.Derive(1), ast.Typedefs, opt)
			if err != nil {
				return nil, err
			}
			buf.NewLine(1)
		}
		ast.ServiceDef.GenOpt(buf.Derive(1))
		buf.WriteLine("}")
	}

	result := make(map[string]string)
	result[currentName] = buf.String()
	for subPkgName, includeFile := range ast.Includes {
		codes, err := includeFile.CodeGen(subPkgName, lang, false, opt)
		if err != nil {
			return nil, err
		}
		for k, v := range codes {
			result[k] = v
		}
	}
	return result, nil
}

func (srvOpt ServiceType) GenOpt(buf *IdentBuffer) {
	version := GetAnnotation(srvOpt.Annotations, "version").UnwrapOrString("0.0.1")
	servName := GetAnnotation(srvOpt.Annotations, "servName").UnwrapOrString("")
	servType := GetAnnotation(srvOpt.Annotations, "servType").UnwrapOrString("http")
	signType := GetAnnotation(srvOpt.Annotations, "signType").UnwrapOrString("")

	retry := strings.Trim(GetAnnotation(srvOpt.Annotations, "retry").UnwrapOrString(""), `"`)
	retryCount := strings.Trim(GetAnnotation(srvOpt.Annotations, "retryCount").UnwrapOrString(""), `"`)
	minHealthyRatio := strings.Trim(GetAnnotation(srvOpt.Annotations, "minHealthyRatio").UnwrapOrString(""), `"`)
	healthyThreshold := strings.Trim(GetAnnotation(srvOpt.Annotations, "healthyThreshold").UnwrapOrString(""), `"`)
	maxCooldownTime := strings.Trim(GetAnnotation(srvOpt.Annotations, "maxCooldownTime").UnwrapOrString(""), `"`)

	buf.WriteLine("option (dirpc.service_opt) = {")
	ibuf := buf.Derive(1)
	ibuf.WriteLine(fmt.Sprintf("version: %s", version))
	if servName != "" {
		ibuf.WriteLine(fmt.Sprintf("servName: %s", servName))
	}
	ibuf.WriteLine(fmt.Sprintf("servType: %s", servType))
	if signType != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "signType", signType))
	}
	httpOptions := getHTTPOption(srvOpt.Annotations)
	for _, opt := range httpOptions {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", opt.Key, opt.Val))
	}
	if retry != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "retry", retry))
	} else if retryCount != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "retry", retryCount))
	}

	if minHealthyRatio != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "minHealthyRatio", minHealthyRatio))
	}

	if healthyThreshold != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "healthyThreshold", healthyThreshold))
	}

	if maxCooldownTime != "" {
		ibuf.WriteLine(fmt.Sprintf("%s: %s", "maxCooldownTime", maxCooldownTime))
	}

	buf.WriteLine("};")
}

func getHTTPOption(ants []AnnotationField) []KVPair {
	timeoutMsec := strings.Trim(GetAnnotation(ants, "timeoutMsec").UnwrapOrString(""), `"`)
	connectTimeoutMsec := strings.Trim(GetAnnotation(ants, "connectTimeoutMsec").UnwrapOrString(""), `"`)
	sendTimeoutMsec := strings.Trim(GetAnnotation(ants, "sendTimeoutMsec").UnwrapOrString(""), `"`)
	recvTimeoutMsec := strings.Trim(GetAnnotation(ants, "recvTimeoutMsec").UnwrapOrString(""), `"`)
	contentType := GetAnnotation(ants, "contentType").UnwrapOrString("")
	options := []KVPair{}
	if timeoutMsec != "" {
		_, err := strconv.Atoi(timeoutMsec)
		if err == nil {
			options = append(options, KVPair{
				Key: "timeoutMsec",
				Val: timeoutMsec,
			})
		}
	}
	if connectTimeoutMsec != "" {
		_, err := strconv.Atoi(connectTimeoutMsec)
		if err == nil {
			options = append(options, KVPair{
				Key: "connectTimeoutMsec",
				Val: connectTimeoutMsec,
			})
		}
	}
	if sendTimeoutMsec != "" {
		_, err := strconv.Atoi(sendTimeoutMsec)
		if err == nil {
			options = append(options, KVPair{
				Key: "sendTimeoutMsec",
				Val: sendTimeoutMsec,
			})
		}
	}
	if recvTimeoutMsec != "" {
		_, err := strconv.Atoi(recvTimeoutMsec)
		if err == nil {
			options = append(options, KVPair{
				Key: "recvTimeoutMsec",
				Val: recvTimeoutMsec,
			})
		}
	}
	if contentType != "" {
		options = append(options, KVPair{
			Key: "contentType",
			Val: contentType,
		})
	}
	return options
}

type IdentBuffer struct {
	ident   int
	buf     *bytes.Buffer
	newLine bool
	lines   int
}

func NewIdentWriter(ident int) *IdentBuffer {
	return &IdentBuffer{
		ident:   ident,
		buf:     bytes.NewBuffer(nil),
		newLine: true,
	}
}

func (ibf *IdentBuffer) String() string {
	return ibf.buf.String()
}

func (ibf *IdentBuffer) WriteLine(s string) {
	ibf.WriteString(strings.TrimRight(s, "\n"))
	ibf.NewLine(1)
}

func (ibf *IdentBuffer) NewLine(n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		ibf.buf.WriteByte('\n')
	}
	ibf.lines += n
	ibf.newLine = true
}

func (ibf *IdentBuffer) tab() {
	if ibf.newLine {
		for i := 0; i < ibf.ident; i++ {
			ibf.buf.WriteByte('\t')
		}
		ibf.newLine = false
	}
}

func (ibf *IdentBuffer) WriteString(s string) {
	ibf.tab()
	ibf.buf.WriteString(s)
}

func (ibf *IdentBuffer) WriteByte(b byte) {
	if b == '\n' {
		ibf.NewLine(1)
		return
	}
	ibf.tab()
	ibf.buf.WriteByte(b)
}

func (ibf *IdentBuffer) Derive(addIdent int) *IdentBuffer {
	return &IdentBuffer{
		ident:   ibf.ident + addIdent,
		buf:     ibf.buf,
		newLine: true,
	}
}

func singleLineFormat(s string) string {
	if strings.Index(s, "//") != -1 {
		s = strings.TrimPrefix(s, "//")
		s = strings.TrimSpace(s)
		return fmt.Sprintf(" //%s", s)
	}

	return s
}

func multiLineFormat(s string) string {
	if strings.Index(s, "/*") != -1 {
		cArr := strings.Split(s, "\n")
		for i := range cArr {
			if strings.Index(cArr[i], "/**") != -1 {
				cArr[i] = strings.TrimPrefix(cArr[i], "/**")
			} else if strings.Index(cArr[i], "/*") != -1 {
				cArr[i] = strings.TrimPrefix(cArr[i], "/*")
			}
			cArr[i] = strings.TrimSuffix(cArr[i], "*/")
			cArr[i] = strings.TrimSpace(cArr[i])
		}

		return fmt.Sprintf("//%s", strings.TrimSpace(strings.Join(cArr, " ")))
	}

	return s
}

func sortStructDefs(structDefs map[string]StructType) []StructType {
	sortedStructdefs := make([]StructType, len(structDefs))
	for _, sd := range structDefs {
		sortedStructdefs[sd.SequenceNum] = sd
	}
	return sortedStructdefs
}

// SortFunctionDefs ...
func SortFunctionDefs(functionDefs map[string]FunctionType) []FunctionType {
	sortedFunctiondefs := make([]FunctionType, len(functionDefs))
	for _, fd := range functionDefs {
		sortedFunctiondefs[fd.SequenceNum] = fd
	}
	return sortedFunctiondefs
}

func sortEnumDefs(enumDefs map[string]EnumType) []EnumType {
	sortedEnumDefs := make([]EnumType, len(enumDefs))
	for _, ed := range enumDefs {
		sortedEnumDefs[ed.SequenceNum] = ed
	}
	return sortedEnumDefs
}

func GetAnnotation(annotations []AnnotationField, key string) util.Option {
	for _, v := range annotations {
		if v.Key == key {
			return v.Value
		}
	}
	return util.None()
}

func GetAnnotationString(annotations []AnnotationField, key string) string {
	v := GetAnnotation(annotations, key)
	return v.UnwrapOrString("")
}

func GetAnnotationTrimString(annotations []AnnotationField, key string) string {
	return strings.Trim(GetAnnotationString(annotations, key), `"`)
}

func GetAnnotationInt(annotations []AnnotationField, key string) int {
	s := GetAnnotationTrimString(annotations, key)
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

type DirSearcher struct {
	Dirs []string
}

func NewDirSearcher(dirs []string) IncludeSearcher {
	ds := &DirSearcher{}
	containCurrent := false
	for _, dir := range dirs {
		if dir == "." {
			containCurrent = true
		}
		ds.Dirs = append(ds.Dirs, dir)
	}
	if !containCurrent {
		ds.Dirs = append(ds.Dirs, ".")
	}
	return ds
}

func (ds *DirSearcher) Open(fName string) (string, error) {
	for _, d := range ds.Dirs {
		fileName := filepath.Join(d, fName)
		fd, err := os.Open(fileName)
		if err == nil {
			data, err := ioutil.ReadAll(fd)
			if err != nil {
				return "", fmt.Errorf("fail to open file: %s, due to %s", fileName, err)
			}
			return string(data), nil
		}
	}
	return "", fmt.Errorf("cannot find %s in any onf the %v", fName, ds.Dirs)
}
