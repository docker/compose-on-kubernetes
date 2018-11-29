# kubectl plugin

1. Create a directory ~/.kube/plugins/composectl

```
$ mkdir -p ~/.kube/plugins/composectl
```

2. Copy the content below in ~/.kube/plugins/composectl/plugin.yaml

```yaml
name: "composectl"
shortDesc: "Manage Docker compose files"
command: "path-to-composectl"
```

3. Use the cli as usual

```
$ kubectl plugin composectl ls
NAMESPACE	NAME	SERVICES	AGE
default		samples	5		2h
```

Note: if you want to pass options to the cli, you need to use `--` as a separator.

```
$ kubectl plugin composectl ls -- -o wide
NAMESPACE	NAME	SERVICES	AGE	STATUS
default		samples	5		2h	Stack is started
```