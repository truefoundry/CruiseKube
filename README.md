# cruisekube

## PostgreSQL Configuration

Cruisekube supports two modes for PostgreSQL database connectivity:

### 1. Local Mode (Default)
When no external PostgreSQL connection is provided, cruisekube will deploy a PostgreSQL instance within the Kubernetes cluster.

### 2. External Mode
When PostgreSQL connection details are provided, cruisekube will connect to your existing PostgreSQL instance.

## Configuration Options

### External PostgreSQL Connection

To use an external PostgreSQL instance, configure the connection in your `values.yaml`:

#### Option 1: Connection URL (Recommended)
```yaml
postgresql:
  connection:
    url: "postgresql://username:password@host:port/database"
```

#### Option 2: Individual Parameters
```yaml
postgresql:
  connection:
    host: "your-postgres-host.com"
    port: 5432
    database: "cruisekube"
    username: "your-username"
    password: "your-password"
    sslMode: "require"  # Options: disable, allow, prefer, require, verify-ca, verify-full
```

### Local PostgreSQL Configuration

When using local mode, you can customize the PostgreSQL deployment:

```yaml
postgresql:
  local:
    enabled: true  # Automatically managed based on connection.url
    image:
      repository: postgres
      tag: "15-alpine"
      pullPolicy: IfNotPresent
    
    # Database configuration
    database: "cruisekube"
    username: "cruisekube"
    password: "cruisekube123"  # Change this in production
    
    # Storage configuration
    persistence:
      enabled: true
      size: 10Gi
      storageClass: ""
      accessMode: ReadWriteOnce
    
    # Resource limits
    resources:
      limits:
        cpu: 500m
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 128Mi
```

## Examples

### Using External PostgreSQL
```yaml
postgresql:
  connection:
    url: "postgresql://myuser:mypass@postgres.example.com:5432/cruisekube_db"
```

### Using Local PostgreSQL with Custom Settings
```yaml
postgresql:
  local:
    database: "my_cruisekube_db"
    username: "admin"
    password: "secure-password-123"
    persistence:
      size: 20Gi
      storageClass: "fast-ssd"
    resources:
      limits:
        cpu: 1000m
        memory: 1Gi
```

## Security Considerations

- **Change default passwords**: Always change the default PostgreSQL password in production environments
- **Use SSL**: Enable SSL mode when connecting to external PostgreSQL instances
- **Network policies**: Consider implementing network policies to restrict database access
- **Backup strategy**: Implement appropriate backup strategies for your PostgreSQL data
