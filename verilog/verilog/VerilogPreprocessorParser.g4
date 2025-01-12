// Author: Mustafa Said Ağca
// License: MIT

parser grammar VerilogPreprocessorParser;

options { tokenVocab=VerilogLexer; }

// START SYMBOL
source_text
	: compiler_directive*
	;

grave_accent
	: GRAVE_ACCENT
	| SOURCE_GRAVE_ACCENT
	;

// 19. Compiler directives

compiler_directive
	: celldefine_compiler_directive
	| endcelldefine_compiler_directive
	| default_nettype_compiler_directive
	| text_macro_definition
	| text_macro_usage
	| undefine_compiler_directive
	| ifdef_directive
	| ifndef_directive
	| include_compiler_directive
	| resetall_compiler_directive
	| line_compiler_directive
	| timescale_compiler_directive
	| unconnected_drive_compiler_directive
	| nounconnected_drive_compiler_directive
	| pragma
	| keywords_directive
	| endkeywords_directive
	;

// 19.1 `celldefine and `endcelldefine

celldefine_compiler_directive
	: grave_accent DIRECTIVE_CELLDEFINE
	;

endcelldefine_compiler_directive
	: grave_accent DIRECTIVE_ENDCELLDEFINE
	;

// 19.2 `default_nettype

default_nettype_compiler_directive
	: grave_accent DIRECTIVE_DEFAULT_NETTYPE default_nettype_value
	;

default_nettype_value
	: DIRECTIVE_IDENTIFIER
	;

// 19.3 `define and `undef
// 19.3.1 `define

text_macro_definition
	: grave_accent DIRECTIVE_DEFINE text_macro_identifier (MACRO_TEXT | MACRO_BACKSLASH_NEWLINE)*
	;

text_macro_usage
	: grave_accent text_macro_identifier DIRECTIVE_LIST_OF_ARGUMENTS?
	;

text_macro_identifier
	: TEXT_MACRO_NAME
	| DIRECTIVE_IDENTIFIER
	| CONDITIONAL_MACRO_NAME
	;

// 19.3.2 `undef

undefine_compiler_directive
	: grave_accent DIRECTIVE_UNDEF text_macro_identifier
	;

// 19.4 `ifdef, `else, `elsif, `endif , `ifndef

ifdef_directive
	: grave_accent DIRECTIVE_IFDEF text_macro_identifier SOURCE_TEXT elsif_directive* else_directive? endif_directive
	;

ifndef_directive
	: grave_accent DIRECTIVE_IFNDEF text_macro_identifier SOURCE_TEXT elsif_directive* else_directive? endif_directive
	;

elsif_directive
	: grave_accent DIRECTIVE_ELSIF text_macro_identifier SOURCE_TEXT
	;

else_directive
	: grave_accent DIRECTIVE_ELSE SOURCE_TEXT
	;

endif_directive
	: grave_accent DIRECTIVE_ENDIF
	;

// 19.5 `include

include_compiler_directive
	: grave_accent DIRECTIVE_INCLUDE filename
	;

filename
	: DIRECTIVE_STRING
	;

// 19.6 `resetall

resetall_compiler_directive
	: grave_accent DIRECTIVE_RESETALL
	;

// 19.7 `line

line_compiler_directive
	: grave_accent DIRECTIVE_LINE line_number filename line_level
	;

line_number
	: DIRECTIVE_NUMBER
	;

line_level
	: DIRECTIVE_NUMBER
	;

// 19.8 `timescale

timescale_compiler_directive
	: grave_accent DIRECTIVE_TIMESCALE time_literal DIRECTIVE_SLASH time_literal
	;

time_literal
	: time_number time_unit
	;

time_number
	: DIRECTIVE_NUMBER
	;

time_unit
	: DIRECTIVE_IDENTIFIER
	;

// 19.9 `unconnected_drive and `nounconnected_drive

unconnected_drive_compiler_directive
	: grave_accent DIRECTIVE_UNCONNECTED_DRIVE unconnected_drive_value
	;

unconnected_drive_value
	: DIRECTIVE_IDENTIFIER
	;

nounconnected_drive_compiler_directive
	: grave_accent DIRECTIVE_NOUNCONNECTED_DRIVE
	;

// 19.10 `pragma

pragma
	: grave_accent DIRECTIVE_PRAGMA pragma_name (pragma_expression (DIRECTIVE_COMMA pragma_expression)*)?
	;

pragma_name
	: DIRECTIVE_IDENTIFIER
	;

pragma_expression
	: pragma_keyword (DIRECTIVE_EQUAL pragma_value)?
	| pragma_value
	;

pragma_value
	: DIRECTIVE_IDENTIFIER
	| DIRECTIVE_NUMBER
	| DIRECTIVE_STRING
	;

pragma_keyword
	: DIRECTIVE_IDENTIFIER
	;

// 19.11 `begin_keywords, `end_keywords

keywords_directive
	: grave_accent DIRECTIVE_BEGIN_KEYWORDS version_specifier
	;

version_specifier
	: DIRECTIVE_STRING
	;

endkeywords_directive
	: grave_accent DIRECTIVE_END_KEYWORDS
	;
