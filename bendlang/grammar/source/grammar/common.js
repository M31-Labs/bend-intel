// Common grammar rules between syntaxes

const { PREC, SEMICOLON } = require('./constants');
const { commaSep1, optionalCommaSep1, sep1 } = require('./utils');

module.exports = {
  // Misc
  // ====

  // Compiler-generated and HVM-facing definitions may begin with an
  // underscore (for example `_Tree.fold`), but a lone underscore remains the
  // wildcard token used by patterns.
  _normal_identifier: _ => /(?:[a-zA-Z][A-Za-z0-9_\-.]*|_[A-Za-z0-9_\-.]+)/,

  // Identifier without two consecutive underscores __
  _top_level_identifier: _ => /(?:[a-zA-Z][A-Za-z0-9_\-.]*(?:_[A-Za-z0-9_\-.]+)*|_[A-Za-z0-9_\-.]+)/,

  _id: $ => choice($._normal_identifier, $._top_level_identifier),

  identifier: $ => prec(PREC.call, choice(
    seq(
      // `$.path` is an external symbol, it's the same as an identifier
      // but it must contain a slash '/' in its last lookahead
      repeat1(seq(choice($.path, alias($.path_expr, $.path)), '/')),
      field('name', alias($._id, $.identifier))
    ),
    field('name', $._id)
  )),

  // Expression-only paths keep the external path token from competing with
  // type/group alternatives in return expressions.
  path_identifier: $ => prec(PREC.call, seq(
    repeat1(seq($.path_expr, '/')),
    field('name', alias($._id, $.identifier)),
  )),

  expression_identifier: $ => choice(
    $.path_identifier,
    alias($._id, $.identifier),
  ),

  multiline_comment: _ => token(prec(PREC.multiline_comment, seq('#{', /([^#]|\#[^}])*/, '#}'))),
  comment: _ => token(prec(PREC.comment, seq('#', /(\\+(.|\r?\n)|[^\\\n])*/))),

  // TODO: Parse HVM code
  hvm_code: _ => /.*\n/,

  parameters: $ => seq(
    '(',
    optional($._parameters),
    ')',
  ),

  _parameters: $ => seq(
    commaSep1($.parameter),
    optional(',')
  ),

  // Imperative parameters may carry the same optional type annotations as
  // the compiler parser.  Keeping the annotation in the CST gives the
  // semantic layer a stable range for signature help and type hovers.
  parameter: $ => seq(
    field('name', choice($.identifier, alias('_', $.identifier))),
    optional(seq(':', field('type', $.type_expr)))
  ),


  type_parameters: $ => seq(
    '(',
    optional(commaSep1($.identifier)),
    ')'
  ),

  // Bend has two surface spellings for types: constructor application with
  // parentheses (`List(T)`) and juxtaposition (`List T`).  This intentionally
  // remains a structural grammar; the compiler remains the authority for
  // arity and type validity.
  type_expr: $ => prec.right(1, choice(
    seq($.type_term, '->', $.type_expr),
    $.type_term
  )),

  type_term: $ => prec.left(2, seq(
    $.type_atom,
    repeat($.type_atom)
  )),

  type_atom: $ => choice(
    $.type_call,
    $.type_tuple,
    $.type_group,
    $.identifier,
    alias('_', $.type_hole)
  ),

  type_call: $ => prec(PREC.call, seq(
    field('name', $.identifier),
    '(',
    optional(commaSep1($.type_expr)),
    ')'
  )),

  type_tuple: $ => seq(
    '(',
    $.type_expr,
    ',',
    commaSep1($.type_expr),
    optional(','),
    ')'
  ),

  type_group: $ => seq('(', $.type_expr, ')'),

  unscoped_var: $ => seq('$', alias($.identifier, 'name')),

  // Literals
  // ========

  _literals: $ => choice(
    $.nat,
    $.integer,
    $.float,
    $.character,
    $.string,
    $.symbol,
  ),

  symbol: _ => token(seq('`', /[a-zA-Z0-9+/]{0,4}/, '`')),
  character: _ => token(seq(
    '\'',
    repeat(choice(
      /[^'\\\n]/,
      /\\u\{[0-9A-Fa-f]+\}/,
      /\\./,
    )),
    '\'',
  )),
  string: _ => token(seq('"', repeat(choice(/[^"\\\n]/, /\\./)), '"')),

  integer: _ => token(choice(
    // decimal
    /[+-]?([0-9]+_?)+/,
    // hexadecimal
    /[+-]?(0x|0X)([A-Fa-f0-9]+_?)+/,
    // binary
    /[+-]?(0b|0B)([0-1]+_?)+/,
  )),

  float: _ =>
    token(choice(
      // decimal
      /[+-]?([0-9_]+)\.([0-9_]+)/,
      // hexadecimal
      /[+-]?(0x|0X)([A-Fa-f0-9_]+)\.([A-Fa-f0-9_]+)/,
      // binary
      /[+-]?(0b|0B)([0-1_]+)\.([0-1_]+)/
    )),
}
