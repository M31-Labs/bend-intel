; Definitions and references for editor tag indexes.
(imp_function_definition name: (identifier) @name) @definition.function
(object_definition name: (identifier) @name) @definition.class
(imp_type_definition name: (identifier) @name) @definition.type
(fun_type_definition name: (identifier) @name) @definition.type
(hvm_definition name: (identifier) @name) @definition.function
(call_expression (identifier) @reference.call)
(constructor (identifier) @reference.constructor)
