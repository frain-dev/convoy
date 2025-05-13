package plugins

import (
	"fmt"
	"strings"
	"sync"
)

type ModuleID string

type Module interface {
	ModuleInfo() ModuleInfo
}

// ModuleInfo represents a registered module.
type ModuleInfo struct {
	ID  ModuleID
	New func() Module
}

// Name returns the Name (last element) of a module ID.
func (id ModuleID) Name() string {
	if id == "" {
		return ""
	}
	parts := strings.Split(string(id), ".")
	return parts[len(parts)-1]
}

func (mi ModuleInfo) String() string { return string(mi.ID) }

func RegisterModule(instance Module) {
	mod := instance.ModuleInfo()

	if mod.ID == "" {
		panic("module ID missing")
	}
	if mod.ID == "core" || mod.ID == "admin" {
		panic(fmt.Sprintf("module ID '%s' is reserved", mod.ID))
	}
	if mod.New == nil {
		panic("missing ModuleInfo.New")
	}
	if val := mod.New(); val == nil {
		panic("ModuleInfo.New must return a non-nil module instance")
	}

	mdMutex.Lock()
	defer mdMutex.Unlock()

	if _, ok := modules[(mod.ID)]; ok {
		panic(fmt.Sprintf("module already registered: %s", mod.ID))
	}
	modules[(mod.ID)] = mod
}

// GetModule returns module information from its ID (full name).
func GetModule(name ModuleID) (ModuleInfo, error) {
	mdMutex.RLock()
	defer mdMutex.RUnlock()
	m, ok := modules[name]
	if !ok {
		return ModuleInfo{}, fmt.Errorf("module not registered: %s", name)
	}
	return m, nil
}

// GetModules returns the modules which implement interface name
// func GetModules() []ModuleID {
// 	var m []ModuleID
//
// 	for id := range modules {
// 		if _, ok := modules[id].New().(Emitter); ok {
// 			m = append(m, id)
// 		}
// 	}
//
// 	return m
// }

func ErrModuleValidation(id ModuleID, field string) error {
	return fmt.Errorf("module %s is not loaded. field: %s", id, field)
}

type Loader interface {
	Load(ModuleContext) error
}

type Validator interface {
	Validate() error
}

type Cleaner interface {
	Cleanup() error
}

var (
	mdMutex sync.RWMutex
	modules = make(map[ModuleID]ModuleInfo)
)
