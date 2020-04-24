# Design patterns and conventions

## Error Handling

Errors are either handled locally, within a method or logged and returned. The custom `Logger` ensures that the error is
logged once. Subsequence `Error()` and `Trace()` higher up the stack are ignored.

This ensures:
- The logged stack trace reflects _where_ the error occurred.
- Errors are always handled or logged.
- Consistency makes PR review easier.

Errors that cannot be handled locally are deemded _unrecoverable_ and are logged and returned to the
`Reconcile()`.

The reconciler will:
- Log the error
- Set a `ReconcileFailed` condition.
- Re-queue the event.

## Orgainization

All constructs should be organized, scoped, and named based on a specific topic or concern. Constructs 
named _uitl_, _helper_, _misc_ are __highly__ discouraged as they are an anti-pattern. Everything should
fit within an appropriately named: package, .go (file), struct, function. Thoughtful organization and naming
reflects a thoughful design.

### Packages

Packages should have a narrowly focused concern and be placed in the heirarchy as _locally_ as possible.

Top level infrastructure packages:

#### apis

Provides kubernetes API types.

The `model.go` provides convenience functions to fetch k8s resources and CRs.

The `resource.go` provides the `MigResource` interface.  ALL of the CRs implement this interface
which defines common behavior.

The `labels.go` provides support for _correlation_ labels which are used to correlate resources created by
a controller to one of our CRs.

#### controller

Provides controllers.

#### logging

Provides a custom logger that supports de-duplication of logged errors. In addition, it provides
a `Trace()` method which is like `Error()` but does not require a _message_.  The logger includes
a short header in the form of: `<name>|<short digest>: <message>`.  The _digest_ is updated on each
`Reset()` and provides a means to correlate all of the entries for the call chain (such as a 
specific reconcile).  The Logger also filters out error=`ConflictError` entries as they are
considered noisy and unhelpful. 

Example:
```
if err != nil {
    log.Trace(err)
    return err
}
```

The `Logger.Reset()` must be called at the beginning of each call chain. This is usually
the `Reconciler.Reconcile()`.  The `Reset()`:

#### compat

Provides k8s compatability. This includes a custom `Client` which performs automatic type
conversion to/from the cluster based on the cluster's version.  The `Client` also implements the
`DiscoveryInterface` and includes the REST `Config`; cluster version `Major`, `Minor`. To use
these extended capabilities, the client must be type-asseted.

Example:
```
dClient := client.(dapi.DiscoveryInterface)
```

#### settings

Provides application settings. The global `Settings` object loads and includes
settings primarily from environment variables.  All settings are scoped by concern.
- **Role** - Manager roles.
- **Proxy** - Manager proxy settings.
- **Plan** - Plan controller settings.
- **Migration** - Migration controller settings.

#### reference

Provides support for CR references. The global `Map` correlates resoruces referenced by
`ObjectReference` fields on the CR to the CR itself.  When watched using the provided
watch event `Handler`, a reconcile event is queue for the (owner) CR instead of an event
for the watched (target) resource.

#### remote

Provides support for _watching_ resources on _other_ clusters. This includes a
mapping of a special `Manager` that watches resources on the remote cluster to
a specific `Cluster`. When a watch event is received, a reconcile event is queued
to the `Cluster` controller reconciler.  The `Map` is managed by the `Cluster`
controller. 
 

#### pods

Provides support for Pod actions such as: `PodExec`.

## Reconciler

Each controller provides a `Reconciler` which has a _main_ method named `Reconcile()`.
In an effort to keep this method maintainable, it delegates all application logic to
a method defined in a separate .go file. Each `Reconcile()` has the standard anatomy:
- Logger.Reset()
- Fetch the resource.
- Begin condition staging.
- Perform validation (call r.validate() defined in validation.go).
- Reconcile (delegate to methods).
- End condition staging.
- Mark as reconciled (See: ObservedGeneration|ObservedDigest)
- Update the resource.

On _error_, the reconciler will:
- Log the error.
- Return `ReconcileResult{Requeue: true}`

### Validation

The `validation.go` file contains a `validate() error` method which performs
validations. Each discrete validation is delegated to separate method and roughly
corresponds to a specific condition (or group of conditions). Since all conditions
have been _unstaged_, the validation only needs to set conditions. They do not
need to delete (clear) them. In the event that a validation is skipped, the related
condition should be _re-staged_.

### Conditions

Each CR status includes the `Conditions` collection and the `Condition` object. The
collection is basically a list of `Condition` that provides enhanced functionality.
It also introduces the concept of _staging_. The goal is to preserve conditions across
reconciles. This preserves the condition timestamps and support durable conditions
 and _re-staging_ when validations are skipped.

A condition is _set_ using `SetCondition()`:
```
cr.Status.SetCondition(migapi.Condition{
    Type:     SomeCondition,
    Status:   True,
    Reason:   NotSet,
    Category: Critical,
    Message:  "Something happened.",
})
```

A condition is _re-staged_ using `StageCondition()`:
```
cr.Status.StageCondition(SomeCondition)
```

A condition may be marked as `Durable: true` which means it's never un-unstaged.
Durable conditions must be explicitly deleted using `DeleteCondition()`.

The `Condition.Items` array may be used to list details about the condition. The
`Message` field may contains `[]` which is substituted with the `Items` when
_staging_ ends.

All `Conditions` methods are idempotent and most support _varargs_.
