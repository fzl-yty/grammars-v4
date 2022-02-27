package thrift

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"git.xiaojukeji.com/gulfstream/thriftpp/parser"
	"git.xiaojukeji.com/gulfstream/thriftpp/util"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/emirpasic/gods/stacks"
	"github.com/emirpasic/gods/stacks/arraystack"
)

type ThriftListener struct {
	*parser.BaseTParserListener
	stack               stacks.Stack
	enumStack           stacks.Stack
	fieldsStack         stacks.Stack
	annotationStack     stacks.Stack
	typedefs            map[string]TypedefType
	structDefs          map[string]StructType
	functionDefs        map[string]FunctionType // 当一个idl文件中不同service存在相同function时不可用
	enumDefs            map[string]EnumType
	constDefs           map[string]ConstType
	serviceDef          ServiceType
	serviceDefs         []ServiceType
	includes            []IncludeName
	namespaces          []NameSpace
	tokens              *antlr.CommonTokenStream
	includeListeners    map[string]*ThriftListener
	functionDefsService map[string][]FunctionType // map[serviceName][]FunctionType 临时变量 用于构建ServiceType.FunctionDefs
}

func NewThriftListener(tokens *antlr.CommonTokenStream) *ThriftListener {
	return &ThriftListener{
		stack:               arraystack.New(),
		fieldsStack:         arraystack.New(),
		annotationStack:     arraystack.New(),
		enumStack:           arraystack.New(),
		typedefs:            make(map[string]TypedefType),
		structDefs:          make(map[string]StructType),
		functionDefs:        make(map[string]FunctionType),
		enumDefs:            make(map[string]EnumType),
		constDefs:           make(map[string]ConstType),
		tokens:              tokens,
		includeListeners:    make(map[string]*ThriftListener),
		functionDefsService: make(map[string][]FunctionType),
	}
}

func (tl *ThriftListener) copyInclude() AnalysisResult {
	result := AnalysisResult{}
	result.EnumDefs = tl.enumDefs
	result.FunctionDefs = tl.functionDefs
	result.Namespaces = tl.namespaces
	result.ServiceDef = tl.serviceDef
	result.ServiceDefs = tl.serviceDefs
	result.StructDefs = tl.structDefs
	result.Typedefs = tl.typedefs
	result.ConstDefs = tl.constDefs
	result.IncludeNames = tl.includes
	result.Includes = make(map[string]AnalysisResult)
	for refName, v := range tl.includeListeners {
		result.Includes[refName] = v.copyInclude()
	}
	return result
}

func (tl *ThriftListener) ExitInclude(ctx *parser.IncludeContext) {
	file := ctx.LITERAL().GetText()
	tl.includes = append(tl.includes, IncludeName{
		Name:    strings.Trim(file, `"`),
		Context: ctx,
	})
}

func (tl *ThriftListener) ExitNamespace(ctx *parser.NamespaceContext) {
	scope := ""
	name := ""
	if ctx.ASTERISK() != nil {
		scope = "*"
	} else {
		scope = ctx.IDENTIFIER(0).GetText()
	}
	if lit := ctx.LITERAL(); lit != nil {
		name = lit.GetText()
	} else {
		if ctx.ASTERISK() != nil {
			name = ctx.IDENTIFIER(0).GetText()
		} else {
			name = ctx.IDENTIFIER(1).GetText()
		}
	}
	tl.namespaces = append(tl.namespaces, NameSpace{
		Scope:   scope,
		Name:    name,
		Context: ctx,
	})
}

func (tl *ThriftListener) ExitService(ctx *parser.ServiceContext) {
	name := ctx.IDENTIFIER(0).GetText()

	functionDefs := make(map[string]FunctionType)
	if functions, ok := tl.functionDefsService[name]; ok {
		for _, function := range functions {
			function.SequenceNum = len(functionDefs)
			functionDefs[function.Name] = function
		}
	}

	serviceDef := ServiceType{
		Context:      ctx,
		Name:         name,
		Annotations:  tl.getAnnotations(),
		Comments:     tl.newComments(ctx),
		FunctionDefs: functionDefs,
	}
	if tl.serviceDef.Name == "" {
		tl.serviceDef = serviceDef
	}
	tl.serviceDefs = append(tl.serviceDefs, serviceDef)
}

func (tl *ThriftListener) getAnnotations() []AnnotationField {
	var arr []AnnotationField
	for !tl.annotationStack.Empty() {
		v, _ := tl.annotationStack.Pop()
		field, _ := v.(AnnotationField)
		arr = append(arr, field)
	}
	// reverse
	n := len(arr)
	left := 0
	right := n - 1
	for left < right {
		arr[left], arr[right] = arr[right], arr[left]
		left += 1
		right -= 1
	}
	return arr
}

func (tl *ThriftListener) ExitBase_type(_ *parser.Base_typeContext) {
	v, _ := tl.stack.Pop()
	realType, _ := v.(RealBaseType)
	tl.stack.Push(BaseType{
		Inner:       string(realType),
		Annotations: tl.getAnnotations(),
	})
}

func (tl *ThriftListener) ExitContainer_type(_ *parser.Container_typeContext) {
	v, _ := tl.stack.Pop()
	tl.stack.Push(ContainerType{
		Inner:       v.(Type),
		Annotations: tl.getAnnotations(),
	})
}

func (tl *ThriftListener) ExitStruct_def(ctx *parser.Struct_defContext) {
	name := ctx.IDENTIFIER().GetText()
	var fieldTypes []StructField
	usedNum := map[int]bool{}
	for !tl.fieldsStack.Empty() {
		v, _ := tl.fieldsStack.Pop()
		field, _ := v.(StructField)

		if _, ok := usedNum[field.FieldID]; ok {
			panic(fmt.Sprintf("field id : %d duplicated in struct %s", field.FieldID, name))
		}
		usedNum[field.FieldID] = true

		fieldTypes = append(fieldTypes, field)
	}
	// reverse
	n := len(fieldTypes)
	left := 0
	right := n - 1
	for left < right {
		fieldTypes[left], fieldTypes[right] = fieldTypes[right], fieldTypes[left]
		left += 1
		right -= 1
	}
	st := StructType{
		Name:        name,
		SequenceNum: len(tl.structDefs),
		FieldType:   fieldTypes,
		Context:     ctx,
		Annotations: tl.getAnnotations(),
		Comments:    tl.newComments(newDoubleEnd(ctx.GetStart(), ctx.LPARENT().GetSymbol())),
	}
	tl.structDefs[name] = st
}

func (tl *ThriftListener) ExitField(ctx *parser.FieldContext) {
	fieldType, _ := tl.stack.Pop()
	if ctx.Field_id() == nil {
		panic(fmt.Sprintf("field number is required for %s", ctx.IDENTIFIER().GetText()))
	}
	fid, _ := strconv.Atoi(ctx.Field_id().GetStart().GetText())

	fieldReq := FieldReq_Required
	req := ctx.Field_req()
	if req != nil && req.GetStart().GetText() == string(FieldReq_Optional) {
		fieldReq = FieldReq_Optional
	}

	var (
		defaultValue    interface{}
		hasDefaultValue bool
	)
	equal := ctx.EQUAL()
	if equal != nil {
		hasDefaultValue = true
		defaultValue = ctx.Const_value().GetText()
	}

	tl.fieldsStack.Push(StructField{
		FieldReq:        fieldReq,
		Name:            ctx.IDENTIFIER().GetText(),
		Context:         ctx,
		FieldID:         fid,
		Type:            fieldType.(Type),
		Annotations:     tl.getAnnotations(),
		Comments:        tl.newComments(ctx),
		DefaultValue:    defaultValue,
		HasDefaultValue: hasDefaultValue,
	})
}

func (tl *ThriftListener) ExitEnum_rule(ctx *parser.Enum_ruleContext) {
	var enumFields []EnumField
	for !tl.enumStack.Empty() {
		v, _ := tl.enumStack.Pop()
		enumFields = append(enumFields, v.(EnumField))
	}
	// reverse
	n := len(enumFields)
	left := 0
	right := n - 1
	for left < right {
		enumFields[left], enumFields[right] = enumFields[right], enumFields[left]
		left += 1
		right -= 1
	}
	ident := ctx.IDENTIFIER().GetText()
	tl.enumDefs[ident] = EnumType{
		Name:        ident,
		SequenceNum: len(tl.enumDefs),
		Enumerate:   enumFields,
		Context:     ctx,
		Annotations: tl.getAnnotations(),
		Comments:    tl.newComments(ctx),
	}
}

func (tl *ThriftListener) ExitEnum_field(ctx *parser.Enum_fieldContext) {
	key := ctx.IDENTIFIER().GetText()
	val := 0
	if token := ctx.Integer(); token != nil {
		val, _ = strconv.Atoi(token.GetText())
	}
	tl.enumStack.Push(EnumField{
		Key:         key,
		Value:       val,
		Context:     ctx,
		Annotations: tl.getAnnotations(),
		Comments:    tl.newComments(ctx),
	})
}

func (tl *ThriftListener) ExitSenum(_ *parser.SenumContext) {
	// clear annotation
	tl.annotationStack.Clear()
}

func (tl *ThriftListener) ExitUnion(_ *parser.UnionContext) {
	// clear annotation and field
	tl.annotationStack.Clear()
	tl.fieldsStack.Clear()
}

func (tl *ThriftListener) ExitException(_ *parser.ExceptionContext) {
	// clear annotation and field
	tl.annotationStack.Clear()
	tl.fieldsStack.Clear()
}

func (tl *ThriftListener) ExitTypedef(ctx *parser.TypedefContext) {
	ident := ctx.IDENTIFIER().GetText()
	originalType, _ := tl.stack.Pop()
	tl.typedefs[ident] = TypedefType{
		Name:         ident,
		SequenceNum:  len(tl.typedefs),
		OriginalType: originalType.(Type),
		Context:      ctx,
		Annotations:  tl.getAnnotations(),
		Comments:     tl.newComments(ctx),
	}
}

//nolint
func (tl *ThriftListener) ExitConst_rule(ctx *parser.Const_ruleContext) {
	var constType = ConstType{}
	name := ctx.IDENTIFIER().GetText()
	constType.Name = name
	constType.Type = ctx.Field_type().GetText()
	constType.Flag = ctx.KW_CONST().GetText()
	constType.Value = ctx.Const_value().GetText()
	constType.Context = ctx
	constType.Comments = tl.newComments(ctx)
	tl.constDefs[name] = constType
}

func (tl *ThriftListener) ExitReal_base_type(ctx *parser.Real_base_typeContext) {
	tl.stack.Push(NewRealBaseType(ctx.GetText()))
}

func (tl *ThriftListener) ExitField_type(ctx *parser.Field_typeContext) {
	if ident := ctx.IDENTIFIER(); ident != nil {
		tl.stack.Push(NewUDFType(ident.GetText()))
	}
}

func (tl *ThriftListener) ExitSet_type(_ *parser.Set_typeContext) {
	innerType, _ := tl.stack.Pop()
	tl.stack.Push(NewSetType(innerType.(Type)))
}

func (tl *ThriftListener) ExitList_type(_ *parser.List_typeContext) {
	innerType, _ := tl.stack.Pop()
	tl.stack.Push(NewListType(innerType.(Type)))
}

func (tl *ThriftListener) ExitMap_type(_ *parser.Map_typeContext) {
	value, _ := tl.stack.Pop()
	key, _ := tl.stack.Pop()
	tl.stack.Push(NewMapType(key.(Type), value.(Type)))
}

func (tl *ThriftListener) ExitType_annotation(ctx *parser.Type_annotationContext) {
	key := ctx.IDENTIFIER().GetText()
	value := util.None()
	if eq := ctx.EQUAL(); eq != nil {
		v, _ := tl.stack.Pop()
		if inner, ok := v.(Integer); ok {
			value = util.Some(inner.Val)
		} else if inner, ok := v.(Literal); ok {
			value = util.Some(inner.Val)
		}
	}
	tl.annotationStack.Push(AnnotationField{
		Key:      key,
		Value:    value,
		Comments: tl.newComments(ctx),
	})
}

func (tl *ThriftListener) ExitAnnotation_value(ctx *parser.Annotation_valueContext) {
	if lit := ctx.LITERAL(); lit != nil {
		tl.stack.Push(Literal{Val: lit.GetText()})
	} else if litS := ctx.LITERAL_STRING(); litS != nil {
		tl.stack.Push(Literal{Val: litS.GetText()})
	} else {
		intV, ok := ctx.Integer().(*parser.IntegerContext)
		if !ok {
			panic("v isn't the type of *parser.IntegerContext")
		}
		if v := intV.INTEGER(); v != nil {
			value, _ := strconv.Atoi(v.GetText())
			tl.stack.Push(Integer{
				Val:  value,
				Bits: 10,
			})
		} else {
			value, _ := strconv.ParseInt(intV.HEX_INTEGER().GetText(), 16, 64)
			tl.stack.Push(Integer{
				Val:  int(value),
				Bits: 16,
			})
		}
	}
}

func (tl *ThriftListener) ExitFunction(ctx *parser.FunctionContext) {
	s := ctx.GetParent()
	serviceName := s.(*parser.ServiceContext).IDENTIFIER(0).GetText()
	responseType, _ := tl.stack.Pop()
	var fieldType []StructField
	for !tl.fieldsStack.Empty() {
		v, _ := tl.fieldsStack.Pop()
		fieldType = append(fieldType, v.(StructField))
	}
	// reverse
	n := len(fieldType)
	left := 0
	right := n - 1
	for left < right {
		fieldType[left], fieldType[right] = fieldType[right], fieldType[left]
		left += 1
		right -= 1
	}
	name := ctx.IDENTIFIER().GetText()
	functionType := FunctionType{
		Name:        name,
		ServiceName: serviceName,
		SequenceNum: len(tl.functionDefs),
		Response:    responseType.(Type),
		Requests:    fieldType,
		Annotations: tl.getAnnotations(),
		Context:     ctx,
		Comments:    tl.newComments(ctx),
	}

	tl.functionDefs[name] = functionType
	functionType.SequenceNum = len(tl.functionDefs) - 1
	tl.functionDefs[name] = functionType
	tl.functionDefsService[serviceName] = append(tl.functionDefsService[serviceName], functionType)
}

func (tl *ThriftListener) EnterThrows_list(_ *parser.Throws_listContext) {
	tl.fieldsStack.Push(nil)
}

// 这里的throw把函数的参数也给清除了
func (tl *ThriftListener) ExitThrows_list(_ *parser.Throws_listContext) {
	for !tl.fieldsStack.Empty() {
		v, _ := tl.fieldsStack.Pop()
		if v == nil {
			break
		}
	}
	tl.annotationStack.Clear()
}

func (tl *ThriftListener) resolveTypes(currentType interface{}, typedefs, enums, structs *hashset.Set) {
	switch t := currentType.(type) {
	case UDFType:
		typeName := string(t)
		if originalType, ok := tl.typedefs[typeName]; ok {
			if !typedefs.Contains(typeName) {
				typedefs.Add(typeName)
				tl.resolveTypes(originalType.OriginalType, typedefs, enums, structs)
			}
		} else if structInfo, ok := tl.structDefs[typeName]; ok {
			if !structs.Contains(typeName) {
				structs.Add(typeName)
				for _, field := range structInfo.FieldType {
					tl.resolveTypes(field.Type, typedefs, enums, structs)
				}
			}
		} else if _, ok := tl.enumDefs[typeName]; ok {
			if !enums.Contains(typeName) {
				enums.Add(typeName)
			}
		}
	case ContainerType:
		tl.resolveTypes(t.Inner, typedefs, enums, structs)
	case SetType:
		tl.resolveTypes(t.Inner, typedefs, enums, structs)
	case ListType:
		tl.resolveTypes(t.Inner, typedefs, enums, structs)
	case MapType:
		tl.resolveTypes(t.Key, typedefs, enums, structs)
		tl.resolveTypes(t.Value, typedefs, enums, structs)
	case BaseType:
		// do nothing
	}
}

type IncludeSearcher interface {
	Open(file string) (string, error)
}

func parseIDL(idl string, searchDir IncludeSearcher, searched sets.Set, singleMode bool) (result *ThriftListener, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("%v", r)
		}
	}()
	input := antlr.NewInputStream(idl)
	lexer := &bailErrorLexer{parser.NewTLexer(input)}
	tokens := antlr.NewCommonTokenStream(lexer, 0)
	p := parser.NewTParser(tokens)
	p.BuildParseTrees = true
	p.RemoveErrorListeners()
	p.AddErrorListener(new(ParseCancelException))
	document := p.Document()
	listener := NewThriftListener(tokens)

	antlr.ParseTreeWalkerDefault.Walk(listener, document)
	if !singleMode {
		includes := make(map[string]struct{})
		for _, f := range listener.includes {
			includes[f.Name] = struct{}{}
		}
		if len(includes) > 0 && searchDir == nil {
			return nil, errors.New("null IncludeSearcher")
		}
		for includeFile := range includes {
			fileName := filepath.Base(includeFile)
			idx := strings.IndexByte(fileName, '.')
			if idx == -1 {
				return nil, fmt.Errorf("illegal include path: %s", includeFile)
			}
			refName := fileName[:idx]
			if searched.Contains(includeFile) {
				return nil, fmt.Errorf("cycled include %s", includeFile)
			}
			subIDL, err := searchDir.Open(includeFile)
			if err != nil {
				return nil, fmt.Errorf("fail to load include file: %s, due to %s", includeFile, err)
			}
			searched.Add(includeFile)
			subListener, err := parseIDL(subIDL, searchDir, searched, singleMode)
			if err != nil {
				return nil, fmt.Errorf("fail to parse idl: %s, due to %s", includeFile, err)
			}
			searched.Remove(includeFile)
			listener.includeListeners[refName] = subListener
		}
	}

	finalErr = dirpcFieldsCheck(listener)

	return listener, finalErr
}

func Parse(idls []string) (AnalysisResult, error) {
	retAst := AnalysisResult{}
	for _, idl := range idls {
		listener, err := parseIDL(idl, nil, nil, true)
		if err != nil {
			return retAst, err
		}

		curAst := listener.copyInclude()

		if curAst.ServiceDef.Name != "" {
			retAst.ServiceDef = curAst.ServiceDef
		}

		if len(curAst.ServiceDefs) > 0 {
			retAst.ServiceDefs = curAst.ServiceDefs
		}

		if retAst.ConstDefs == nil {
			retAst.ConstDefs = curAst.ConstDefs
		} else {
			for k, v := range curAst.ConstDefs {
				retAst.ConstDefs[k] = v
			}
		}

		if retAst.Typedefs == nil {
			retAst.Typedefs = curAst.Typedefs
		} else {
			for k, v := range curAst.Typedefs {
				retAst.Typedefs[k] = v
			}
		}

		if retAst.StructDefs == nil {
			retAst.StructDefs = curAst.StructDefs
		} else {
			for k, v := range curAst.StructDefs {
				retAst.StructDefs[k] = v
			}
		}

		if retAst.FunctionDefs == nil {
			retAst.FunctionDefs = curAst.FunctionDefs
		} else {
			for k, v := range curAst.FunctionDefs {
				retAst.FunctionDefs[k] = v
			}
		}

		if retAst.EnumDefs == nil {
			retAst.EnumDefs = curAst.EnumDefs
		} else {
			for k, v := range curAst.EnumDefs {
				retAst.EnumDefs[k] = v
			}
		}

		if retAst.Namespaces == nil {
			retAst.Namespaces = curAst.Namespaces
		} else {
			for _, v := range curAst.Namespaces {
				retAst.Namespaces = append(retAst.Namespaces, v)
			}
		}

		if retAst.Includes == nil {
			retAst.Includes = curAst.Includes
		} else {
			for k, v := range curAst.Includes {
				retAst.Includes[k] = v
			}
		}

		if retAst.IncludeNames == nil {
			retAst.IncludeNames = curAst.IncludeNames
		} else {
			for _, v := range curAst.IncludeNames {
				retAst.IncludeNames = append(retAst.IncludeNames, v)
			}
		}
	}
	return retAst, nil
}

func Analysis(idl string, dirs []string, singleMode bool) (result AnalysisResult, finalErr error) {
	listener, err := parseIDL(idl, NewDirSearcher(dirs), hashset.New(), singleMode)
	if err != nil {
		return AnalysisResult{}, err
	}
	result = listener.copyInclude()
	if finalErr != nil {
		return AnalysisResult{}, finalErr
	}
	return result, nil
}

type bailErrorLexer struct {
	*parser.TLexer
}

func (bl *bailErrorLexer) Recover(r antlr.RecognitionException) {
	panic(r)
}

func getMultiLineComment(point antlr.Token, tokens *antlr.CommonTokenStream) util.Option {
	comments := tokens.GetHiddenTokensToLeft(point.GetTokenIndex(), parser.TLexerML_COMMENT_CHAN)
	n := len(comments)
	if n == 0 {
		return util.None()
	}
	buf := bytes.NewBuffer(nil)
	for _, line := range comments {
		buf.WriteString(line.GetText())
		buf.WriteByte('\n')
	}
	return util.Some(buf.String())
}

func getSingleLineComment(start antlr.Token, point antlr.Token, tokens *antlr.CommonTokenStream) util.Option {
	commentsRight := tokens.GetHiddenTokensToRight(point.GetTokenIndex(), parser.TLexerSL_COMMENT_CHAN)
	commentsAbove := tokens.GetHiddenTokensToLeft(start.GetTokenIndex(), parser.TLexerSL_COMMENT_CHAN)
	if len(commentsRight) == 0 && len(commentsAbove) == 0 {
		return util.None()
	}

	buf := bytes.NewBuffer(nil)

	// 如果注释不是在当前行后面，则舍弃，不算作当前行的注释
	if len(commentsRight) > 0 {
		isMyComment := true
		firstComment := commentsRight[0]
		wsLeft := tokens.GetHiddenTokensToLeft(firstComment.GetTokenIndex(), parser.TLexerWS_CHAN)
		if len(wsLeft) > 0 {
			for _, ws := range wsLeft {
				if strings.Contains(ws.GetText(), "\n") {
					isMyComment = false
					break
				}
			}
		}
		if isMyComment {
			buf.WriteString(firstComment.GetText())

			return util.Some(buf.String())
		}
	}

	// 如果上面的注释和当前行之间有换行，则舍弃，不算作当前行的注释
	if len(commentsAbove) > 0 {
		isMyComment := true

		lastComment := commentsAbove[len(commentsAbove)-1]
		wsBelow := tokens.GetHiddenTokensToRight(lastComment.GetTokenIndex(), parser.TLexerWS_CHAN)
		if len(wsBelow) > 0 {
			for _, ws := range wsBelow {
				if strings.Contains(ws.GetText(), "\n") {
					isMyComment = false
					break
				}
			}
		}
		if isMyComment {
			buf.WriteString(lastComment.GetText())

			return util.Some(buf.String())
		}
	}

	return util.None()
}

type doubleEnd interface {
	GetStart() antlr.Token
	GetStop() antlr.Token
}

type doubleEndImpl struct {
	start, stop antlr.Token
}

func (d doubleEndImpl) GetStart() antlr.Token {
	return d.start
}

func (d doubleEndImpl) GetStop() antlr.Token {
	return d.stop
}

func newDoubleEnd(start, stop antlr.Token) doubleEnd {
	return doubleEndImpl{
		start: start,
		stop:  stop,
	}
}

func (tl *ThriftListener) newComments(d doubleEnd) Comments {
	start := d.GetStart()
	multi := getMultiLineComment(start, tl.tokens)
	end := d.GetStop()
	single := getSingleLineComment(start, end, tl.tokens)
	return Comments{
		MultiLine:  multi.UnwrapOr("").(string),
		SingleLine: single.UnwrapOr("").(string),
	}
}

type ParseCancelException struct {
	*antlr.DefaultErrorListener
}

func (ex *ParseCancelException) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	panic("line " + strconv.Itoa(line) + ":" + strconv.Itoa(column) + " " + msg)
}

const (
	serverTypeHttp   = "http"
	serverTypeHttps  = "https"
	serverTypeGrpc   = "grpc"
	serverTypeThrift = "thrift"
)

const (
	httpMethodPost = "POST"
	httpMethodGet  = "GET"
)

var (
	checkDiRPCFlag = true
	lock           sync.Mutex
)

// nolint
func SetCheckDiRPC(flag bool) {
	lock.Lock()
	checkDiRPCFlag = flag
	lock.Unlock()
}

func dirpcFieldsCheck(listener *ThriftListener) error {

	// 当前检查的idl文件没有定义service，略过下面的检查
	if listener.serviceDef.Name == "" {
		return nil
	}

	if !checkDiRPCFlag {
		return nil
	}

	servType := GetAnnotationTrimString(listener.serviceDef.Annotations, "servType")
	if servType == "" {
		return errors.New("missing required field: servType")
	}

	version := GetAnnotationTrimString(listener.serviceDef.Annotations, "version")
	if version == "" {
		return errors.New("missing required field: version")
	}

	servName := GetAnnotationTrimString(listener.serviceDef.Annotations, "servName")
	if servName == "" {
		return errors.New("missing required field: servName")
	}

	serverTimeout := GetAnnotationTrimString(listener.serviceDef.Annotations, "timeoutMsec")
	if serverTimeout != "" {
		_, err := strconv.Atoi(serverTimeout)
		if err != nil {
			return errors.New("illegal server timeout")
		}
	}

	serverConnectTimeout := GetAnnotationTrimString(listener.serviceDef.Annotations, "connectTimeoutMsec")
	if serverConnectTimeout != "" {
		_, err := strconv.Atoi(serverConnectTimeout)
		if err != nil {
			return errors.New("illegal server connect timeout")
		}
	}

	serverSendTimeout := GetAnnotationTrimString(listener.serviceDef.Annotations, "sendTimeoutMsec")
	if serverSendTimeout != "" {
		_, err := strconv.Atoi(serverSendTimeout)
		if err != nil {
			return errors.New("illegal server send timeout")
		}
	}

	serverRecvTimeout := GetAnnotationTrimString(listener.serviceDef.Annotations, "recvTimeoutMsec")
	if serverRecvTimeout != "" {
		_, err := strconv.Atoi(serverRecvTimeout)
		if err != nil {
			return errors.New("illegal server recv timeout")
		}
	}

	for funcName, funcDef := range listener.functionDefs {
		timeout := GetAnnotationTrimString(funcDef.Annotations, "timeoutMsec")
		if timeout != "" {
			_, err := strconv.Atoi(timeout)
			if err != nil {
				return fmt.Errorf("illegal timeout for function %s", funcName)
			}
		}

		connectTimeout := GetAnnotationTrimString(funcDef.Annotations, "connectTimeoutMsec")
		if connectTimeout != "" {
			_, err := strconv.Atoi(connectTimeout)
			if err != nil {
				return fmt.Errorf("illegal connect timeout for function %s", funcName)
			}
		}

		sendTimeout := GetAnnotationTrimString(funcDef.Annotations, "sendTimeoutMsec")
		if sendTimeout != "" {
			_, err := strconv.Atoi(sendTimeout)
			if err != nil {
				return fmt.Errorf("illegal send timeout for function %s", funcName)
			}
		}

		recvTimeout := GetAnnotationTrimString(funcDef.Annotations, "recvTimeoutMsec")
		if recvTimeout != "" {
			_, err := strconv.Atoi(recvTimeout)
			if err != nil {
				return fmt.Errorf("illegal recv timeout for function %s", funcName)
			}
		}

		switch servType {
		case serverTypeHttp, serverTypeHttps:
			method := strings.ToUpper(GetAnnotationTrimString(funcDef.Annotations, "httpMethod"))
			if method != httpMethodPost && method != httpMethodGet {
				return fmt.Errorf("illegal or missing required field httpMethod for function %s", funcName)
			}

			path := GetAnnotationTrimString(funcDef.Annotations, "path")
			if path == "" {
				return fmt.Errorf("missing required field path for function %s", funcName)
			}

			contentType := GetAnnotationTrimString(funcDef.Annotations, "contentType")
			if contentType == "" {
				return fmt.Errorf("missing required field contentType for function %s", funcName)
			}

			// flattenform 是为了兼容以前的idl，新生成的idl不应该有这个
			if contentType != "form" && contentType != "json" && contentType != "flattenform" {
				return fmt.Errorf("invalid contentType: %s, only support \"form\" or \"json\"", contentType)
			}

		case serverTypeGrpc:
			if timeout == "" && serverTimeout == "" {
				return fmt.Errorf("missing timeout for function %s", funcName)
			}

			if connectTimeout == "" && serverConnectTimeout == "" {
				return fmt.Errorf("missing connect timeout for function %s", funcName)
			}

		case serverTypeThrift:
			if sendTimeout == "" && serverSendTimeout == "" {
				return fmt.Errorf("missing send timeout for function %s", funcName)
			}

			if recvTimeout == "" && serverRecvTimeout == "" {
				return fmt.Errorf("missing recv timeout for function %s", funcName)
			}
		}

	}
	return nil
}
