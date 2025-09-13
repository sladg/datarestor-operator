kubectl delete tasks.backup.datarestor-operator.com --all -A
kubectl get tasks.backup.datarestor-operator.com -A -o name | xargs -r -n1 kubectl patch --type=merge -p '{"metadata":{"finalizers":null}}'


NS=default
for r in $(kubectl api-resources --verbs=list --namespaced -o name); do
  kubectl -n "$NS" get "$r" -o name --ignore-not-found \
  | xargs -n1 -I{} kubectl -n "$NS" patch {} --type=merge -p '{"metadata":{"finalizers":[]}}' >/dev/null 2>&1 || true
done