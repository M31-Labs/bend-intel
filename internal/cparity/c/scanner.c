#include "tree_sitter/array.h"
#include "tree_sitter/parser.h"

#include <assert.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>

enum TokenType {
    NEWLINE,
    INDENT,
    DEDENT,
    COMMENT,
    NAT,
    PATH,
    PATH_EXPR,
    ERROR_SENTINEL
};

typedef struct {
    Array(uint16_t) indents;
} Scanner;


static inline bool identifier_char(uint32_t curr) {
    return (('0' <= curr && curr <= '9') ||
            ('a' <= curr && curr <= 'z') ||
            ('A' <= curr && curr <= 'Z') ||
            (curr == '.' || curr == '-' || curr == '_'));
}

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

bool tree_sitter_bend_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
    Scanner *scanner = (Scanner *)payload;

    if (valid_symbols[ERROR_SENTINEL]) {
        return false;
    }

    lexer->mark_end(lexer);

    bool found_end_of_line = false;
    uint32_t indent_length = 0;
    int32_t first_comment_indent_length = -1;

    // printf("analyzing '%c', comment valid: %d\n", lexer->lookahead == '\n' ? 'N' : lexer->lookahead, valid_symbols[COMMENT]);

    for (;;) {
        if (lexer->lookahead == '\n') {
            found_end_of_line = true;
            indent_length = 0;
            skip(lexer);
        } else if (lexer->lookahead == ' ') {
            indent_length++;
            skip(lexer);
        } else if (lexer->lookahead == '\r' || lexer->lookahead == '\f') {
            indent_length = 0;
            skip(lexer);
        } else if (lexer->lookahead == '\t') {
            indent_length += 8;
            skip(lexer);
        } else if (lexer->lookahead == '#' && (valid_symbols[INDENT] || valid_symbols[DEDENT] ||
                                               valid_symbols[NEWLINE] || valid_symbols[NAT])) {
            // A hash followed immediately by a digit is a functional natural
            // literal. Consume the marker for lookahead; returning false
            // lets the regular grammar tokenise the literal from the
            // original byte.
            lexer->advance(lexer, false);
            if (lexer->lookahead == '{') {
                // `#{...#}` is a multiline comment handled by the regular
                // grammar token, not the indentation/comment scanner.
                return false;
            }
            if ('0' <= lexer->lookahead && lexer->lookahead <= '9') {
                if (!valid_symbols[NAT]) {
                    return false;
                }
                while ('0' <= lexer->lookahead && lexer->lookahead <= '9') {
                    advance(lexer);
                }
                lexer->result_symbol = NAT;
                lexer->mark_end(lexer);
                return true;
            }
            // If we haven't found an EOL yet, this is a comment after an
            // expression. Do not generate an indentation token for it.
            if (!found_end_of_line) {
                return false;
            }
            if (first_comment_indent_length == -1) {
                first_comment_indent_length = (int32_t)indent_length;
            }
            while (lexer->lookahead && lexer->lookahead != '\n') {
                skip(lexer);
            }
            skip(lexer);
            indent_length = 0;
        } else if (lexer->eof(lexer)) {
            indent_length = 0;
            found_end_of_line = true;
            break;
        } else {
            break;
        }
    }

    if (found_end_of_line) {
        if (scanner->indents.size > 0) {
            uint16_t current_indent_length = *array_back(&scanner->indents);

            if (valid_symbols[INDENT] && indent_length > current_indent_length) {
                array_push(&scanner->indents, indent_length);
                lexer->result_symbol = INDENT;
                return true;
            }

            if ((valid_symbols[DEDENT] || !valid_symbols[NEWLINE]) && indent_length < current_indent_length &&

                // Wait to create a dedent token until we've consumed any
                // comments
                // whose indentation matches the current block.
                first_comment_indent_length < (int32_t)current_indent_length) {
                array_pop(&scanner->indents);
                lexer->result_symbol = DEDENT;
                return true;
            }
        }

        if (valid_symbols[NEWLINE]) {
            lexer->result_symbol = NEWLINE;
            return true;
        }
    }

    // Identifiers that end in a slash '/'.
    // The slash is not included in the symbol.
    if (valid_symbols[PATH_EXPR] && valid_symbols[PATH]) {
        if (identifier_char(lexer->lookahead)) {
            advance(lexer);
            while (identifier_char(lexer->lookahead)) {
                advance(lexer);
            }
            if (lexer->lookahead == '/') {
                lexer->mark_end(lexer);
                // Constructor and pattern paths continue with `{` or `:`;
                // keep those on the general PATH token. Expressions such as
                // `List/Nil` use the expression-specific token.
                advance(lexer);
                while (identifier_char(lexer->lookahead)) {
                    advance(lexer);
                }
                if (valid_symbols[PATH] && (lexer->lookahead == '{' || lexer->lookahead == ':')) {
                    lexer->result_symbol = PATH;
                } else {
                    lexer->result_symbol = PATH_EXPR;
                }
                return valid_symbols[lexer->result_symbol];
            }
            return false;
        }
    }

    if (valid_symbols[PATH]) {
        if (identifier_char(lexer->lookahead)) {
            advance(lexer);
            while (identifier_char(lexer->lookahead)) {
                advance(lexer);
            }

            if (lexer->lookahead == '/') {
                lexer->result_symbol = PATH;
                lexer->mark_end(lexer);
                return true;
            }

            return false;
        }
    }


    return false;
}

unsigned tree_sitter_bend_external_scanner_serialize(void *payload, char *buffer) {
    Scanner *scanner = (Scanner *)payload;

    size_t size = 0;

    uint32_t iter = 1;
    for (; iter < scanner->indents.size && size < TREE_SITTER_SERIALIZATION_BUFFER_SIZE; ++iter) {
        buffer[size++] = (char)*array_get(&scanner->indents, iter);
    }

    return size;
}

void tree_sitter_bend_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
    Scanner *scanner = (Scanner *)payload;

    array_delete(&scanner->indents);
    array_push(&scanner->indents, 0);

    if (length > 0) {
        size_t size = 0;
        for (; size < length; size++) {
            array_push(&scanner->indents, (unsigned char)buffer[size]);
        }
    }
}

void *tree_sitter_bend_external_scanner_create(void) {
    Scanner *scanner = calloc(1, sizeof(Scanner));
    array_init(&scanner->indents);
    tree_sitter_bend_external_scanner_deserialize(scanner, NULL, 0);
    return scanner;
}

void tree_sitter_bend_external_scanner_destroy(void *payload) {
    Scanner *scanner = (Scanner *)payload;
    array_delete(&scanner->indents);
    free(scanner);
}
