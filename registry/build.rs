fn main() {
    prost_build::compile_protos(&["../proto/registry.proto"], &["../proto/"]).unwrap();
}
