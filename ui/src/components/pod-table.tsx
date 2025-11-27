import { useMemo } from 'react'
import { IconLoader, IconCircleDottedLetterL, IconCircleDottedLetterC, IconCircleDottedLetterF, IconCircleDottedLetterU } from '@tabler/icons-react'
import { Pod } from 'kubernetes-types/core/v1'
import { Link } from 'react-router-dom'

import { MetricsData, PodHealthData, PodWithMetrics } from '@/types/api'
import { getPodStatus } from '@/lib/k8s'
import { formatDate } from '@/lib/utils'

import { MetricCell } from './metrics-cell'
import { PodStatusIcon } from './pod-status-icon'
import { Column, SimpleTable } from './simple-table'
import { Badge } from './ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from './ui/card'

export function PodTable(props: {
  pods?: PodWithMetrics[]
  health?: PodHealthData
  labelSelector?: string
  isLoading?: boolean
  hiddenNode?: boolean
  allowLink?: boolean
}) {
  const { pods, health, isLoading, allowLink = true } = props

  // Pod table columns
  const podColumns = useMemo(
    (): Column<PodWithMetrics>[] => [
      {
        header: 'Name',
        accessor: (pod: Pod) => pod.metadata,
        cell: (value: unknown) => {
          const meta = value as Pod['metadata']
          return (
            // <div className="font-medium text-blue-500 hover:underline">
            //   <Link to={`/pods/${meta!.namespace}/${meta!.name}`}>
            //     {meta!.name}
            //   </Link>
            // </div>
            <div
              className={
                allowLink
                  ? "font-medium hover:underline"
                  : "font-medium text-foreground"  // no hover, no underline
              }
            >
              {allowLink ? (
                <Link to={`/pods/${meta!.namespace}/${meta!.name}`}>
                  {meta!.name}
                </Link>
              ) : (
                meta!.name
              )}
            </div>
          )
        },
        align: 'left' as const,
      },
      {
        header: 'Raft Role',
        accessor: (pod: Pod) => pod.metadata,
        cell: (value: unknown) => {
          const meta = value as Pod['metadata']
          const podName = meta?.name
          const key = (meta?.namespace || '') + '/' + (podName as string)
          const healthData = props.health?.[key]

          const stateColors = {
            'LEADER': 'bg-blue-200',
            'FOLLOWER': 'bg-green-200',
            'CANDIDATE': 'bg-pink-200',
            'UNKNOWN': 'bg-gray-200',
          }

          if (!healthData) return <Badge variant="outline" className="text-muted-foreground px-1.5">{'Unknown'}</Badge>

          return (
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={`${stateColors[healthData.state] || stateColors.UNKNOWN}`}>{healthData.state}</Badge>
            </div>
          )
        },
      },
      {
        header: 'Healthy',
        accessor: (pod: Pod) => pod.metadata,
        cell: (value: unknown) => {
          const meta = value as Pod['metadata']
          const podName = meta?.name
          const key = (meta?.namespace || '') + '/' + (podName as string)
          const healthData = props.health?.[key]

          if (!healthData) return (
            <Badge variant="outline" className="text-muted-foreground px-1.5">
              {'Unknown'}
            </Badge>
          )

          if (healthData.error) {
            return (
              <Badge variant="destructive" className="text-xs">
                Error: {healthData.error}
              </Badge>
            )
          }

          return (
            <Badge
              variant={healthData.healthy ? 'default' : 'destructive'}
              className={healthData.healthy ? 'bg-green-500' : 'bg-red-500'}
            >
              {healthData.healthy ? 'Healthy' : 'Unhealthy'}
            </Badge>
          )
        },
      },
      {
        header: 'Version',
        accessor: (pod: Pod) => pod.spec,
        cell: (value: unknown) => {
          const spec = value as Pod['spec']
          const version = spec!.containers[0]!.image
          return (
            // <Badge variant="outline" className="text-muted-foreground px-1.5">
            <span className="text-muted-foreground px-1.5">
              {version.replace(/typesense\/typesense:/g, '')}
            </span>
            // </Badge>
          )
        },
      },
      {
        header: 'Ready',
        accessor: (pod: Pod) => {
          const status = getPodStatus(pod)
          return `${status.readyContainers} / ${status.totalContainers}`
        },
        cell: (value: unknown) => value as string,
      },
      {
        header: 'Restart',
        accessor: (pod: Pod) => {
          const status = getPodStatus(pod)
          return status.restartString || '0'
        },
        cell: (value: unknown) => {
          return (
            <span className="text-muted-foreground text-sm">
              {value as number}
            </span>
          )
        },
      },
      {
        header: 'Status',
        accessor: (pod: Pod) => pod,
        cell: (value: unknown) => {
          const status = getPodStatus(value as Pod)
          return (
            <Badge variant="outline" className="text-muted-foreground px-1.5">
              <PodStatusIcon status={status.reason} />
              {status.reason}
            </Badge>
          )
        },
      },
      {
        header: 'CPU',
        accessor: (pod: PodWithMetrics) => {
          return pod.metrics
        },
        cell: (value: unknown) => {
          return <MetricCell type="cpu" metrics={value as MetricsData} />
        },
      },
      {
        header: 'Memory',
        accessor: (pod: PodWithMetrics) => {
          return pod.metrics
        },
        cell: (value: unknown) => {
          return <MetricCell type="memory" metrics={value as MetricsData} />
        },
      },
      // {
      //   header: 'IP',
      //   accessor: (pod: Pod) => pod.status?.podIP || '-',
      //   cell: (value: unknown) => (
      //     <span className="text-sm text-muted-foreground font-mono">
      //       {value as string}
      //     </span>
      //   ),
      // },
      // ...(props.hiddenNode
      //   ? []
      //   : [
      //     {
      //       header: 'Node',
      //       accessor: (pod: Pod) => pod.spec?.nodeName || '-',
      //       cell: (value: unknown) => (
      //         allowLink ? (
      //           <Link
      //             to={`/nodes/${value}`}
      //             className="text-blue-600 hover:text-blue-800 hover:underline"
      //           >
      //             {value as string}
      //           </Link>
      //         ) : (
      //           <span className="text-sm text-muted-foreground font-mono">
      //             {value as string}
      //           </span>
      //         )
      //       ),
      //     },
      //   ]
      // ),
      {
        header: 'Created',
        accessor: (pod: Pod) => pod.metadata?.creationTimestamp || '',
        cell: (value: unknown) => {
          return (
            <span className="text-muted-foreground text-sm">
              {formatDate(value as string, true)}
            </span>
          )
        },
      },
    ],
    [props.hiddenNode]
  )

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <IconLoader className="animate-spin mr-2" />
        Loading pods...
      </div>
    )
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Pods</CardTitle>
      </CardHeader>
      <CardContent>
        <SimpleTable
          data={pods || []}
          columns={podColumns}
          emptyMessage="No pods found"
          pagination={{
            enabled: true,
            pageSize: 20,
            showPageInfo: true,
          }}
        />
      </CardContent>
    </Card>
  )
}
