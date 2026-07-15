; Lexical bindings used by the syntax-first scope index.
(imp_function_definition) @local.scope
(fun_function_definition) @local.scope
(imp_lambda) @local.scope
(fun_lambda) @local.scope

(imp_function_definition
  parameters: (parameters (identifier) @local.definition))
(imp_lambda
  parameters: (_) @local.definition)
(assignment_statement
  pat: (identifier) @local.definition)

(identifier) @local.reference
