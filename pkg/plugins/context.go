package plugins

import (
	"context"
	"fmt"
	"log"
	"reflect"
)

type ModuleContext struct {
	context.Context

	modules map[ModuleID]Module
}

func New(ctx context.Context) (ModuleContext, context.CancelFunc) {
	newCtx := ModuleContext{modules: make(map[ModuleID]Module)}
	c, cancel := context.WithCancel(ctx)
	innerCancel := func() {
		cancel()

		// execute cleanup func mounted on the context
		// for _, f := range ctx.cleanupFuncs {
		// 	f()
		// }

		for name, instance := range newCtx.modules {
			if cu, ok := instance.(Cleaner); ok {
				err := cu.Cleanup()
				if err != nil {
					log.Printf("[ERROR] %s (%p): cleanup: %v", name, instance, err)
				}
			}
		}
	}

	newCtx.Context = c
	return newCtx, innerCancel
}

func (ctx ModuleContext) LoadModuleByID(id ModuleID) (any, error) {
	v, ok := ctx.modules[id]
	if ok {
		return v, nil
	}

	mdMutex.RLock()
	modInfo, ok := modules[id]
	mdMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", id)
	}

	if modInfo.New == nil {
		return nil, fmt.Errorf("module '%s' has no constructor", modInfo.ID)
	}

	val := modInfo.New()

	// value must be a pointer for unmarshalling into concrete type, even if
	// the module's concrete type is a slice or map; New() *should* return
	// a pointer; otherwise unmarshalling errors or panics will occur
	if rv := reflect.ValueOf(val); rv.Kind() != reflect.Ptr {
		log.Printf("[WARNING] ModuleInfo.New() for module '%s' did not return a pointer,"+
			" so we are using reflection to make a pointer instead; please fix this by"+
			" using new(Type) or &Type notation in your module's New() function.", id)
		val = reflect.New(rv.Type()).Elem().Addr().Interface().(Module)
	}

	if val == nil {
		// returned module values are almost always type-asserted
		// before being used, so a nil value would panic; and there
		// is no good reason to explicitly declare null modules in
		// a config; it might be because the user is trying to achieve
		// a result the developer isn't expecting, which is a smell
		return nil, fmt.Errorf("module value cannot be null")
	}

	if prov, ok := val.(Loader); ok {
		err := prov.Load(ctx)
		if err != nil {
			// incomplete provisioning could have left the state
			// dangling, so make sure it gets cleaned up
			if cleaner, ok := val.(Cleaner); ok {
				err2 := cleaner.Cleanup()
				if err2 != nil {
					err = fmt.Errorf("%v; additionally, cleanup: %v", err, err2)
				}
			}
			return nil, fmt.Errorf("load %s: %v", modInfo, err)
		}
	}

	if validator, ok := val.(Validator); ok {
		err := validator.Validate()
		if err != nil {
			// since the module was already provisioned, make sure we clean up
			if cleaner, ok := val.(Cleaner); ok {
				err2 := cleaner.Cleanup()
				if err2 != nil {
					err = fmt.Errorf("%v; additionally, cleanup: %v", err, err2)
				}
			}
			return nil, fmt.Errorf("%s: invalid configuration: %v", modInfo, err)
		}
	}

	ctx.modules[id] = val

	return val, nil
}
