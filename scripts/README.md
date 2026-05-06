# Scripts

## `clusterrequests.py`

```bash
python3 scripts/clusterrequests.py --context <your-context>
python3 scripts/clusterrequests.py --context <your-context> -n <namespace>
python3 scripts/clusterrequests.py --context <your-context> --selector app=<label>
python3 scripts/clusterrequests.py --context <your-context> --original-source manifests -f <manifest-path>
```

| Flag | Description |
| --- | --- |
| `--context` | Kubernetes context to use. |
| `-n`, `--namespace` | Limit to a namespace. Repeatable. |
| `--selector` | Label selector for pods and workloads. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |
| `--original-source` | `cluster` or `manifests`. Default: `cluster`. |
| `-f`, `--manifest` | Manifest file or directory. Required with `--original-source manifests`. |

## `poddbmismatch.py`

```bash
python3 scripts/poddbmismatch.py --context <your-context>
python3 scripts/poddbmismatch.py --config config.local.yaml --context <your-context>
python3 scripts/poddbmismatch.py --context <your-context> --include-completed
```

| Flag | Description |
| --- | --- |
| `--config` | Config file path. Default: `config.yaml`. |
| `--context` | Kubernetes context to use. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |

## `currentreconcile.py`

```bash
python3 scripts/currentreconcile.py --context <your-context>
python3 scripts/currentreconcile.py --config config.local.yaml --context <your-context>
python3 scripts/currentreconcile.py --context <your-context> --include-completed
```

| Flag | Description |
| --- | --- |
| `--config` | Config file path. Default: `config.yaml`. |
| `--context` | Kubernetes context to use. |
| `--include-completed` | Include `Succeeded` and `Failed` pods. |
