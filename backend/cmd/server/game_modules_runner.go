package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// validateGameModuleWASM compiles the module and calls capabilities before install.
// The wazero runtime has no host filesystem or network access.
func validateGameModuleWASM(ctx context.Context, moduleID string, wasm []byte) error {
	if len(wasm) < 8 || !bytes.Equal(wasm[:4], []byte{0x00, 0x61, 0x73, 0x6d}) || !bytes.Equal(wasm[4:8], []byte{0x01, 0x00, 0x00, 0x00}) {
		return errors.New("parser.wasm is not a WebAssembly v1 module")
	}
	if len(wasm) > maxGameModuleWASMBytes {
		return fmt.Errorf("parser.wasm exceeds %d bytes", maxGameModuleWASMBytes)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(maxGameModuleMemoryPages).WithCloseOnContextDone(true))
	defer runtime.Close(ctx)
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return fmt.Errorf("compile parser.wasm: %w", err)
	}
	defer compiled.Close(ctx)
	capabilities, err := callGameModuleWASM(ctx, wasm, moduleID, "capabilities", []byte(`{"abiVersion":"rsm-wasm-json-v1"}`))
	if err != nil {
		return fmt.Errorf("call capabilities: %w", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(capabilities, &decoded); err != nil {
		return fmt.Errorf("decode capabilities response: %w", err)
	}
	return nil
}

func (s *gameModuleService) moduleWASMPath(record gameModuleRecord) string {
	return filepath.Join(s.moduleDir(record.Manifest.ModuleID), record.Manifest.WASMFile)
}

// callWASM marshals one command through the stable JSON ABI used by modules.
func (s *gameModuleService) callWASM(record gameModuleRecord, command string, request any, response any) error {
	wasm, err := os.ReadFile(s.moduleWASMPath(record))
	if err != nil {
		return err
	}
	input, err := json.Marshal(request)
	if err != nil {
		return err
	}
	output, err := callGameModuleWASM(context.Background(), wasm, record.Manifest.ModuleID, command, input)
	if err != nil {
		return err
	}
	if len(output) > maxGameModuleOutputBytes {
		return fmt.Errorf("module output exceeds %d bytes", maxGameModuleOutputBytes)
	}
	if err := json.Unmarshal(output, response); err != nil {
		return fmt.Errorf("decode module %s response: %w", command, err)
	}
	return nil
}

// callGameModuleWASM is the sandbox boundary for parser/editor modules.
// It enforces a timeout, memory cap, bounded output, and the rsm_alloc/rsm_call ABI.
func callGameModuleWASM(ctx context.Context, wasm []byte, moduleID, command string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(maxGameModuleMemoryPages).WithCloseOnContextDone(true))
	defer runtime.Close(ctx)
	_, _ = wasi_snapshot_preview1.Instantiate(ctx, runtime)
	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		return nil, err
	}
	defer compiled.Close(ctx)
	mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, err
	}
	defer mod.Close(ctx)
	alloc := mod.ExportedFunction("rsm_alloc")
	call := mod.ExportedFunction("rsm_call")
	memory := mod.Memory()
	if alloc == nil || call == nil || memory == nil {
		return nil, fmt.Errorf("module %s must export memory, rsm_alloc, and rsm_call", moduleID)
	}
	cmdBytes := []byte(command)
	cmdPtr, err := wasmAlloc(ctx, alloc, memory, cmdBytes)
	if err != nil {
		return nil, err
	}
	inputPtr, err := wasmAlloc(ctx, alloc, memory, input)
	if err != nil {
		return nil, err
	}
	result, err := call.Call(ctx, uint64(cmdPtr), uint64(len(cmdBytes)), uint64(inputPtr), uint64(len(input)))
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, errors.New("rsm_call must return one i64 pointer/length value")
	}
	ptr := uint32(result[0] >> 32)
	length := uint32(result[0] & 0xffffffff)
	if length > maxGameModuleOutputBytes {
		return nil, fmt.Errorf("module response exceeds %d bytes", maxGameModuleOutputBytes)
	}
	data, ok := memory.Read(ptr, length)
	if !ok {
		return nil, errors.New("module response points outside memory")
	}
	return append([]byte(nil), data...), nil
}

func wasmAlloc(ctx context.Context, alloc wazeroFunction, memory wazeroMemory, data []byte) (uint32, error) {
	result, err := alloc.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, err
	}
	if len(result) != 1 {
		return 0, errors.New("rsm_alloc must return one pointer")
	}
	ptr := uint32(result[0])
	if !memory.Write(ptr, data) {
		return 0, errors.New("rsm_alloc returned out-of-range pointer")
	}
	return ptr, nil
}

type wazeroFunction interface {
	Call(context.Context, ...uint64) ([]uint64, error)
}

type wazeroMemory interface {
	Write(uint32, []byte) bool
	Read(uint32, uint32) ([]byte, bool)
}
