# Multi-User Fix Implementation Plan

## CRITICAL: Current Status
- ✅ Send Operations: FIXED (100%)
- ❌ All Other Operations: VULNERABLE (78 locations need fixing)

## Security Issue
Currently User2 can access User1's:
- Profile information, groups, contacts
- Messages (read, delete, edit, star)
- Group management (add/remove members)
- All WhatsApp data and operations

## Required Fixes

### Phase 1: Critical REST Handlers (HIGH PRIORITY)
Update all REST handlers to use AppContext:

1. **ui/rest/user.go** - ✅ PARTIALLY DONE
2. **ui/rest/group.go** - ❌ TODO
3. **ui/rest/message.go** - ❌ TODO
4. **ui/rest/chat.go** - ❌ TODO
5. **ui/rest/newsletter.go** - ❌ TODO

### Phase 2: Critical Usecases (HIGH PRIORITY)
Add client-from-context pattern to all usecases:

1. **usecase/user.go** - ⚠️ IN PROGRESS
2. **usecase/group.go** - ❌ TODO (28+ locations)
3. **usecase/message.go** - ❌ TODO (18 locations)
4. **usecase/chat.go** - ❌ TODO
5. **usecase/newsletter.go** - ❌ TODO

### Phase 3: Helper Functions
Update utility functions to accept client parameter:
- `utils.ValidateJidWithLogin(client, phone)`
- `utils.MustLogin(client)`
- `utils.IsOnWhatsapp(client, phone)`

## Implementation Pattern

### REST Handler Pattern:
```go
func (controller *Handler) Operation(c *fiber.Ctx) error {
    // Create app context with user information
    appCtx := domainApp.NewAppContext(c.UserContext(), c)
    
    response, err := controller.Service.Operation(appCtx, request)
    // ... rest of handler
}
```

### Usecase Pattern:
```go
func (service serviceType) Operation(ctx context.Context, request Request) (Response, error) {
    // Get the appropriate WhatsApp client from context
    client := service.getClientFromContext(ctx)
    if client == nil {
        return response, pkgError.InternalServerError("WhatsApp client not available")
    }
    
    // Use client instead of whatsapp.GetClient()
    result, err := client.SomeOperation()
    // ... rest of usecase
}

// Helper function for each service
func (service serviceType) getClientFromContext(ctx context.Context) *whatsmeow.Client {
    if appCtx, ok := ctx.(*app.AppContext); ok && appCtx.UserID > 0 {
        return whatsapp.GetClientForUser(appCtx.UserID)
    }
    return whatsapp.GetClient() // fallback
}
```

## Testing Strategy
After each fix, test:
1. User1 login & operations work correctly
2. User2 login & operations work correctly  
3. User2 cannot access User1's data
4. User1 cannot access User2's data

## Priority Order
1. **IMMEDIATE**: User & Group operations (most critical)
2. **HIGH**: Message operations
3. **MEDIUM**: Chat & Newsletter operations

## Completion Criteria
- ✅ Zero occurrences of `whatsapp.GetClient()` in business logic
- ✅ All operations use user-specific client
- ✅ Complete user isolation verified
- ✅ No cross-user data access possible
