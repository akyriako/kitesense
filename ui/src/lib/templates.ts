// Kubernetes resource templates

export interface ResourceTemplate {
  name: string
  description: string
  yaml: string
}

export const resourceTemplates: ResourceTemplate[] = [
  {
    name: 'KiND',
    description: 'A basic 3-node cluster on KiND',
    yaml: `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: typesense-local-path
provisioner: rancher.io/local-path
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: kind-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: typesense-local-path
    `,
  },
  {
    name: 'Open Telekom Cloud',
    description: 'A basic 3-node cluster on Open Telekom Cloud',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: otc-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: csi-disk
    `,
  },
  {
    name: 'AWS',
    description: 'A basic 3-node cluster on AWS',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: aws-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: gp2
    `,
  }, {
    name: 'Azure',
    description: 'A basic 3-node cluster on Azure',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: azure-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: managed-csi
    `,
  },
  {
    name: 'GCP',
    description: 'A basic 3-node cluster on GCP',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: gcp-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard-rwo
    `,
  },
  {
    name: 'BYOK',
    description: 'Explicitely define Admin API Key',
    yaml: `apiVersion: v1
kind: Secret
metadata:
  name: typesense-common-bootstrap-key
  namespace: default
type: Opaque
data:
  typesense-api-key: SXdpVG9CcnFYTHZYeTJNMG1TS1hPaGt0dlFUY3VWUloxc1M5REtsRUNtMFFwQU93R1hoanVIVWJLQnE2ejdlSQ==
---
apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: byok
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  adminApiKey:
    name: typesense-common-bootstrap-key
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard,
    `,
  },
   {
    name: 'with Configuration',
    description: 'Add Server Configuration as ENV variables',
    yaml: `apiVersion: v1
kind: ConfigMap
metadata:
  name: server-configuration
  namespace: default
data:
  TYPESENSE_HEALTHY_READ_LAG: "1000"
  TYPESENSE_HEALTHY_WRITE_LAG: "500"
---
apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: srv-conf
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard
  additionalServerConfiguration:
    name: server-configuration
    `,
  },
  {
    name: 'with Ingress',
    description: 'Expose via Ingress+Reverse Proxy',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: public-basic
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard
  ingress:
    ingressClassName: traefik
    host: host.example.com
    clusterIssuer: lets-encrypt-prod
    `,
  },
  {
    name: 'with Ingress+CORS',
    description: 'Expose via Ingress+Reverse Proxy to allowed referers',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: public-cors
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard
  enableCors: true
  corsDomains: "referer1.example.com,referer2.example.com"
  ingress:
    ingressClassName: traefik
    host: host.example.com
    referer: referer1.example.com
    clusterIssuer: lets-encrypt-prod
    `,
  },
  {
    name: 'with Resources',
    description: 'Explicitely Define Requests and Limits',
    yaml: `apiVersion: ts.opentelekomcloud.com/v1alpha1
kind: TypesenseCluster
metadata:
  name: sized
  namespace: default
spec:
  image: typesense/typesense:30.0.rc27-amd64
  replicas: 3
  storage:
    size: 50Mi
    storageClassName: standard
  resources:
    limits:
      cpu: "2000m"
      memory: "4096Mi"
    requests:
      cpu: "100m"
      memory: "64Mi"
    `,
  }
]

export const getTemplateByName = (
  name: string
): ResourceTemplate | undefined => {
  return resourceTemplates.find((template) => template.name === name)
}

export const getTemplateNames = (): string[] => {
  return resourceTemplates.map((template) => template.name)
}
