use bend::fun::Name;
use bend::{
    compile_book,
    diagnostics::{Diagnostics, DiagnosticsConfig, Severity, TextSpan},
    fun::{load_book::load_to_book, Book, SourceKind},
    hvm::hvm_book_show_pretty,
    imports::{BoundSource, Import, ImportType, PackageLoader, Sources},
    CompileOpts,
};
use indexmap::IndexMap;
use serde::{Deserialize, Serialize};
use std::{
    collections::HashMap,
    io::{self, BufRead, Write},
    path::{Path, PathBuf},
};

const PROTOCOL: &str = "bend-intel/1";

#[derive(Debug, Deserialize)]
struct Request {
    protocol: String,
    uri: String,
    #[serde(default, rename = "workspaceRoot")]
    workspace_root: String,
    documents: Vec<Document>,
    #[serde(default, rename = "includeHVM")]
    include_hvm: bool,
}

#[derive(Debug, Deserialize)]
struct Document {
    uri: String,
    #[allow(dead_code)]
    version: i32,
    source: String,
}

#[derive(Debug, Serialize, Default)]
#[serde(rename_all = "camelCase")]
struct Response {
    protocol: &'static str,
    uri: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    diagnostics: Vec<OutDiagnostic>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    types: Vec<TypedSpan>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    signatures: Vec<Signature>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    definitions: Vec<DefinitionOut>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    hvm: String,
}

#[derive(Debug, Serialize, Default)]
#[serde(rename_all = "camelCase")]
struct OutDiagnostic {
    message: String,
    range: Range,
    severity: u8,
    source: String,
}

#[derive(Debug, Serialize, Default)]
struct Range {
    start: Position,
    end: Position,
}

#[derive(Debug, Serialize, Default)]
struct Position {
    line: u32,
    character: u32,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct TypedSpan {
    range: Range,
    #[serde(rename = "type")]
    typ: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct Signature {
    name: String,
    parameters: Vec<String>,
    return_type: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DefinitionOut {
    name: String,
    uri: String,
    range: Range,
}

fn main() {
    let stdin = io::stdin();
    let mut stdout = io::BufWriter::new(io::stdout());
    for line in stdin.lock().lines() {
        let line = match line {
            Ok(line) if !line.trim().is_empty() => line,
            Ok(_) => continue,
            Err(err) => {
                eprintln!("bend-semanticd: read request: {err}");
                break;
            }
        };
        let response = match serde_json::from_str::<Request>(&line) {
            Ok(request) => check(request),
            Err(err) => Response {
                protocol: PROTOCOL,
                uri: String::new(),
                diagnostics: vec![OutDiagnostic {
                    message: format!("invalid semantic request: {err}"),
                    severity: 1,
                    source: "bend-semanticd".into(),
                    ..Default::default()
                }],
                ..Default::default()
            },
        };
        if serde_json::to_writer(&mut stdout, &response).is_err() || writeln!(&mut stdout).is_err()
        {
            break;
        }
        let _ = stdout.flush();
    }
}

fn check(request: Request) -> Response {
    let mut response = Response {
        protocol: PROTOCOL,
        uri: request.uri.clone(),
        ..Default::default()
    };
    if request.protocol != PROTOCOL {
        response.diagnostics.push(OutDiagnostic {
            message: format!("unsupported semantic protocol {}", request.protocol),
            severity: 1,
            source: "bend-semanticd".into(),
            ..Default::default()
        });
        return response;
    }
    let Some(document) = request
        .documents
        .iter()
        .find(|document| document.uri == request.uri)
    else {
        response.diagnostics.push(OutDiagnostic {
            message: "semantic request did not include its target document".into(),
            severity: 1,
            source: "bend-semanticd".into(),
            ..Default::default()
        });
        return response;
    };
    let origin = uri_path(&document.uri, &request.workspace_root);
    let loader = SnapshotLoader::new(&request, &origin);
    let cfg = DiagnosticsConfig::default();
    let mut book = match load_to_book(&origin, &document.source, loader, cfg) {
        Ok(book) => book,
        Err(diagnostics) => {
            response.diagnostics = diagnostics_out(&diagnostics);
            return response;
        }
    };
    let compile = match compile_book(
        &mut book,
        CompileOpts::default(),
        DiagnosticsConfig::default(),
        None,
    ) {
        Ok(result) => {
            if request.include_hvm {
                response.hvm = hvm_book_show_pretty(&result.hvm_book);
            }
            result.diagnostics
        }
        Err(diagnostics) => diagnostics,
    };
    response.diagnostics = diagnostics_out(&compile);
    response_from_book(&mut response, &book, &document.uri);
    response
}

/// Package loader that prefers the editor's in-memory workspace snapshot and
/// falls back to Bend's filesystem loader for unopened imports. This keeps
/// semantic checks coherent while an editor buffer is unsaved, without
/// changing Bend's import semantics.
struct SnapshotLoader {
    documents: HashMap<String, String>,
    fallback: bend::imports::DefaultLoader,
}

impl SnapshotLoader {
    fn new(request: &Request, origin: &Path) -> Self {
        let mut documents = HashMap::new();
        for document in &request.documents {
            documents.insert(
                logical_document_name(&document.uri, &request.workspace_root),
                document.source.clone(),
            );
        }
        Self {
            documents,
            fallback: bend::imports::DefaultLoader::new(origin),
        }
    }
}

impl PackageLoader for SnapshotLoader {
    fn load(&mut self, import: &mut Import) -> Result<Sources, String> {
        let prefix = import.path.as_ref().trim_matches('/');
        let mut sources = Sources::new();
        let mut bound = IndexMap::<Name, Name>::new();
        match &import.imp_type {
            ImportType::Single(file, _) => {
                let key = join_logical_name(prefix, file.as_ref());
                if let Some(code) = self.documents.get(&key) {
                    let name = Name::new(key.clone());
                    sources.insert(name.clone(), code.clone());
                    import.src = BoundSource::File(name);
                }
            }
            ImportType::List(files) => {
                for (file, _) in files {
                    let key = join_logical_name(prefix, file.as_ref());
                    if let Some(code) = self.documents.get(&key) {
                        let name = Name::new(key.clone());
                        sources.insert(name.clone(), code.clone());
                        bound.insert(file.clone(), name);
                    }
                }
                if !bound.is_empty() {
                    import.src = BoundSource::Dir(bound);
                }
            }
            ImportType::Glob => {
                let prefix = prefix.to_string();
                let mut keys: Vec<_> = self
                    .documents
                    .keys()
                    .filter(|key| {
                        if prefix.is_empty() {
                            !key.contains('/')
                        } else {
                            key.strip_prefix(&(prefix.clone() + "/"))
                                .is_some_and(|rest| !rest.contains('/'))
                        }
                    })
                    .cloned()
                    .collect();
                keys.sort();
                for key in keys {
                    if let Some(code) = self.documents.get(&key) {
                        let name = Name::new(key.clone());
                        let file = key.rsplit('/').next().unwrap_or(&key);
                        sources.insert(name.clone(), code.clone());
                        bound.insert(Name::new(file), name);
                    }
                }
                if !bound.is_empty() {
                    import.src = BoundSource::Dir(bound);
                }
            }
        }
        if sources.is_empty() {
            return self.fallback.load(import);
        }
        Ok(sources)
    }
}

fn logical_document_name(uri: &str, root: &str) -> String {
    let path = uri_path(uri, root);
    let mut value = if !root.is_empty() {
        let root_path = Path::new(root);
        path.strip_prefix(root_path)
            .unwrap_or(&path)
            .to_string_lossy()
            .to_string()
    } else {
        path.file_name()
            .map(|name| name.to_string_lossy().to_string())
            .unwrap_or_else(|| path.to_string_lossy().to_string())
    };
    value = value.replace('\\', "/");
    value
        .strip_suffix(".bend")
        .unwrap_or(&value)
        .trim_matches('/')
        .to_string()
}

fn join_logical_name(prefix: &str, file: &str) -> String {
    let file = file.trim_matches('/');
    if prefix.is_empty() {
        file.to_string()
    } else {
        format!("{prefix}/{file}")
    }
}

fn response_from_book(response: &mut Response, book: &Book, uri: &str) {
    for definition in book.defs.values() {
        if !matches!(definition.source.kind, SourceKind::User) {
            continue;
        }
        let name = definition.name.to_string();
        let range = source_range(definition.source.span);
        let typ = definition.typ.to_string();
        response.types.push(TypedSpan {
            range: clone_range(&range),
            typ: typ.clone(),
        });
        response.signatures.push(Signature {
            name: name.clone(),
            parameters: definition
                .rules
                .first()
                .map(|rule| rule.pats.iter().map(|_| "_".to_string()).collect())
                .unwrap_or_default(),
            return_type: typ,
        });
        response.definitions.push(DefinitionOut {
            name,
            uri: uri.to_string(),
            range,
        });
    }
    for definition in book.hvm_defs.values() {
        if !matches!(definition.source.kind, SourceKind::User) {
            continue;
        }
        let name = definition.name.to_string();
        let range = source_range(definition.source.span);
        response.types.push(TypedSpan {
            range: clone_range(&range),
            typ: definition.typ.to_string(),
        });
        response.signatures.push(Signature {
            name: name.clone(),
            parameters: Vec::new(),
            return_type: definition.typ.to_string(),
        });
        response.definitions.push(DefinitionOut {
            name,
            uri: uri.to_string(),
            range,
        });
    }
}

fn diagnostics_out(diagnostics: &Diagnostics) -> Vec<OutDiagnostic> {
    diagnostics
        .diagnostics
        .values()
        .flatten()
        .map(|diagnostic| OutDiagnostic {
            message: diagnostic.message.clone(),
            range: source_range(diagnostic.source.span),
            severity: severity(diagnostic.severity),
            source: "bend-compiler".into(),
        })
        .collect()
}

fn severity(severity: Severity) -> u8 {
    match severity {
        Severity::Error => 1,
        Severity::Warning => 2,
        Severity::Allow => 3,
    }
}

fn source_range(span: Option<TextSpan>) -> Range {
    span.map(|span| Range {
        start: Position {
            line: span.start.line as u32,
            character: span.start.char as u32,
        },
        end: Position {
            line: span.end.line as u32,
            character: span.end.char as u32,
        },
    })
    .unwrap_or_default()
}

fn clone_range(range: &Range) -> Range {
    Range {
        start: Position {
            line: range.start.line,
            character: range.start.character,
        },
        end: Position {
            line: range.end.line,
            character: range.end.character,
        },
    }
}

fn uri_path(uri: &str, root: &str) -> PathBuf {
    if let Some(path) = uri.strip_prefix("file://") {
        return PathBuf::from(percent_decode(path));
    }
    let base = if root.is_empty() { "." } else { root };
    Path::new(base).join("__bend_intel_unsaved__.bend")
}

fn percent_decode(value: &str) -> String {
    let bytes = value.as_bytes();
    let mut out = Vec::with_capacity(value.len());
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'%' && i + 2 < bytes.len() {
            if let (Some(hi), Some(lo)) = (hex_value(bytes[i + 1]), hex_value(bytes[i + 2])) {
                out.push((hi << 4) | lo);
                i += 3;
                continue;
            }
        }
        out.push(bytes[i]);
        i += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

fn hex_value(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}
