# Natively support Compose files on Kubernetes

These are the Compose file features supported on Kubernetes

| Feature                    | docker stack deploy | Compose on Kubernetes  |
|----------------------------|---------------------|------------------------|
| build                      |                     |                        |
|  - context                 |                     |                        |
|  - dockerfile              |                     |                        |
|  - args                    |                     |                        |
|  - cache_from              |                     |                        |
|  - labels                  |                     |                        |
| cap_add                    |                     | Yes                    |
| cap_drop                   |                     | Yes                    |
| command                    | Yes                 | Yes                    |
| configs                    | Yes                 | Yes                    |
| - external                 | Yes                 | Yes                    |
| - file                     | Yes                 | Yes                    |
| - mode                     | Yes                 | Yes                    |
| cgroup_parent              |                     |                        |
| container_name             |                     |                        |
| credential_spec            | Yes                 |                        |
| deploy                     | Yes                 |                        |
| - endpoint_mode: vip       | Yes                 | Yes                    |
| - endpoint_mode: dnsrr     | Yes                 |                        |
| - labels                   | Yes                 | Yes                    |
| - mode                     | Yes                 | Yes                    |
| - placement                | Yes                 | Partial                |
| - replicas                 | Yes                 | Yes                    |
| - resources                | Yes                 | Yes                    |
|   -- limits                | Yes                 | Yes                    |
|   -- reservations          | Yes                 | Yes                    |
| - restart_policy           | Yes                 | Yes                    |
|   -- condition: none       | Yes                 | Yes                    |
|   -- condition: on-failure | Yes                 | Yes                    |
|   -- condition: any        | Yes                 | Yes                    |
|   -- delay                 | Yes                 |                        |
|   -- max_attempts          | Yes                 |                        |
|   -- window                | Yes                 |                        |
| - update_config            | Yes                 |                        |
|   -- parallelism           | Yes                 | Yes                    |
|   -- delay                 | Yes                 |                        |
|   -- failure_action        | Yes                 |                        |
|   -- monitor               | Yes                 |                        |
|   -- max_failure_ratio     | Yes                 |                        |
| - devices                  |                     |                        |
| - depends_on               |                     |                        |
| - dns                      |                     |                        |
| - dns_search               |                     |                        |
| - tmpfs                    |                     | Yes                    |
| - entrypoint               | Yes                 | Yes                    |
| - env_file                 | Yes                 | Yes                    |
| - environment              | Yes                 | Yes                    |
|   -- key=value             | Yes                 | Yes                    |
|   -- key                   | Yes                 |                        |
| - expose                   | Yes                 | Yes                    |
| - external_links           |                     |                        |
| - extra_hosts              | Yes                 | Yes                    |
| - healthcheck              | Yes                 | Yes                    |
|   -- test                  | Yes                 | Yes                    |
|   -- test: NONE            | Yes                 | Yes                    |
|   -- interval              | Yes                 | Yes                    |
|   -- timeout               | Yes                 | Yes                    |
|   -- retries               | Yes                 | Yes                    |
|   -- disable               | Yes                 | Yes                    |
| - image                    | Yes                 | Yes                    |
| - isolation                | Yes                 |                        |
| - labels                   | Yes                 | Yes                    |
| - links                    |                     |                        |
| - logging                  | Yes                 |                        |
|   -- driver                | Yes                 |                        |
|   -- options               | Yes                 |                        |
| - network_mode             |                     |                        |
| - networks                 | Yes                 |                        |
|   -- aliases               | Yes                 |                        |
|   -- ipv4_address          | Yes                 |                        |
|   -- ipv6_address          | Yes                 |                        |
| - pid: host                | Yes                 | Yes                    |
| - ports                    | Yes                 | Yes                    |
|   -- target                | Yes                 | Yes                    |
|   -- published             | Yes                 | Yes                    |
|   -- protocol              | Yes                 | Yes                    |
|   -- mode: host            | Yes                 | Not sure               |
|   -- mode: ingress         | Yes                 | Not sure               |
| - secrets                  | Yes                 | Yes                    |
|   -- source                | Yes                 | Yes                    |
|   -- target                | Yes                 | Yes                    |
|   -- uid                   | Yes                 |                        |
|   -- gid                   | Yes                 |                        |
|   -- mode                  | Yes                 | Yes                    |
| - security_opt             |                     |                        |
| - stop_grace_period        | Yes                 | Yes                    |
| - stop_signal              |                     |                        |
| - sysctls                  |                     |                        |
| - ulimits                  |                     |                        |
| - userns_mode              |                     |                        |
| - volumes                  | Yes                 | Yes                    |
|   -- type: volume          | Yes                 | Yes                    |
|   -- type: bind            | Yes                 | Yes                    |
|   -- source                | Yes                 | Yes                    |
|   -- target                | Yes                 | Yes                    |
|   -- read_only             | Yes                 | Yes                    |
|   -- bind/propagation      | Yes                 |                        |
|   -- volume/nocopy         | Yes                 |                        |
| - restart                  |                     |                        |
| - domainname               |                     |                        |
| - hostname                 | Yes                 | Yes                    |
| - ipc                      |                     | Yes                    |
| - mac_address              |                     |                        |
| - privileged               |                     | Yes                    |
| - read_only                | Yes                 | Yes                    |
| - shm_size                 |                     |                        |
| - stdin_open               | Yes                 | Yes                    |
| - tty                      | Yes                 | Yes                    |
| - user: numerical          | Yes                 | Yes                    |
| - user: name               | Yes                 |                        |
| - working_dir              | Yes                 | Yes                    |

# Not supported by k8s

+ mac_address (https://github.com/kubernetes/kubernetes/issues/2289)
+ shm_size (https://github.com/kubernetes/kubernetes/issues/28272)
+ ulimits (https://github.com/kubernetes/kubernetes/issues/3595)
+ non numerical user

# Placeholders interpolation

Before a yaml file is sent to Kubernenetes, its `${...}` placeholders
are replaced with actual values.
 + A first attempt is made at reading the variable from a `.env` file is
current working directory.
 + If no value is found, an environment variable are used.
