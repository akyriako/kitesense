import { useMemo, useState } from 'react'
import { IconLoader, IconCircleCheckFilled, IconCircleChevronsRightFilled, IconCircleChevronsUpFilled, IconCircleDotFilled, IconAlertTriangleFilled, IconTrash } from '@tabler/icons-react'
import { Pod } from 'kubernetes-types/core/v1'
import { Link } from 'react-router-dom'

import { MetricsData, PodHealthData, PodWithMetrics } from '@/types/api'
import { getPodStatus } from '@/lib/k8s'
import { formatDate2 } from '@/lib/utils'

import { MetricCell } from './metrics-cell'
import { PodStatusIcon } from './pod-status-icon'
import { Column, SimpleTable } from './simple-table'
import { Badge } from './ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from './ui/card'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import { ResourceDeleteConfirmationDialog } from '@/components/resource-delete-confirmation-dialog'

export function PodTable(props: {
  pods?: PodWithMetrics[]
  health?: PodHealthData
  labelSelector?: string
  isLoading?: boolean
  hiddenNode?: boolean
  allowLink?: boolean
  isNested?: boolean
}) {
  const { pods, health, isLoading, allowLink = true, isNested = false } = props
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedPodName, setSelectedPodName] = useState<string>('')
  const [selectedPodNamespace, setSelectedPodNamespace] = useState<string>('')

  // Pod table columns
  const podColumns = useMemo(
    (): Column<PodWithMetrics>[] => [
      {
        header: 'Name',
        accessor: (pod: Pod) => pod,
        cell: (value: unknown) => {
          const pod = value as Pod
          const meta = pod['metadata']
          return (
            <div
              className={
                allowLink
                  ? "font-medium" // keep underline on <Link> itself (cleaner UX)
                  : "font-medium text-blue-500"
              }
            >
              {allowLink ? (
                <Link
                  to={`/pods/${meta?.namespace}/${meta?.name}`}
                  className="hover:underline" // place hover styling on the link itself
                >
                  {meta?.name}
                </Link>
              ) : isNested ? (
                // <Tooltip>
                //   <TooltipTrigger asChild>
                //     {/* asChild hands the trigger to this element */}
                //     <p className="inline-block m-0">{meta?.name}</p>
                //   </TooltipTrigger>
                //   <TooltipContent>
                //     <div className="flex flex-col gap-1">
                //       <div className="flex justify-between">
                //         <span>IP:</span>
                //         <span className="text-right ml-2">{pod.status?.podIP ?? "-"}</span>
                //       </div>
                //       <div className="flex justify-between">
                //         <span>Node:</span>
                //         <span className="text-right ml-2">{pod.spec?.nodeName ?? "-"}</span>
                //       </div>
                //     </div>
                //   </TooltipContent>
                // </Tooltip>
                <span>{meta?.name}</span>

              ) : (
                <span>{meta?.name}</span>
              )}
            </div>
          );
        },
        align: "left" as const,
      },
      {
        header: 'Raft Role',
        accessor: (pod: Pod) => pod.metadata,
        cell: (value: unknown) => {
          const meta = value as Pod['metadata']
          const podName = meta?.name
          const key = (meta?.namespace || '') + '/' + (podName as string)
          const healthData = props.health?.[key]

          if (!healthData) return (<></>)
          // <Badge variant="outline" className="text-muted-foreground px-1.5">{'NOT_AVAILABLE'}</Badge>

          const hState = healthData.state
          // healthData.state.charAt(0).toUpperCase() +
          // healthData.state.slice(1).toLowerCase();

          switch (healthData.state) {
            case 'LEADER':
              return (
                <Badge variant="outline" className="text-muted-foreground px-1.5">
                  <IconCircleCheckFilled className={`fill-blue-400 dark:fill-blue-400 `} />
                  {hState}
                </Badge>
              )
            case 'FOLLOWER':
              return (
                <Badge variant="outline" className="text-muted-foreground px-1.5">
                  <IconCircleChevronsRightFilled className={`fill-amber-200 dark:fill-amber-200 `} />
                  {hState}
                </Badge>
              )
            case 'CANDIDATE':
              return (
                <Badge variant="outline" className="text-muted-foreground px-1.5">
                  <IconCircleChevronsUpFilled className={`fill-pink-400 dark:fill-pink-400 `} />
                  {hState}
                </Badge>
              )
            case 'UNKNOWN':
              return (
                <Badge variant="outline" className="text-muted-foreground px-1.5">
                  <IconCircleDotFilled className={`fill-gray-400 dark:fill-gray-400 `} />
                  {hState}
                </Badge>
              )
            case 'NOT_READY':
              return (
                <Badge variant="outline" className="text-muted-foreground px-1.5">
                  <IconCircleDotFilled className={`fill-gray-400 dark:fill-gray-400 `} />
                  {hState}
                </Badge>
              )
          }
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
            <IconAlertTriangleFilled className="fill-yellow-400 dark:fill-yellow-400 px-0.5 ml-4" />
          )

          if (healthData.error) {
            return (

              <Tooltip>
                <TooltipTrigger asChild>
                  <IconAlertTriangleFilled className="fill-yellow-400 dark:fill-yellow-400 px-0.5 ml-4" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className='overflow-auto max-w-120 whitespace-pre-wrap'>
                    {healthData.error}
                  </p>
                </TooltipContent>
              </Tooltip>
            )
          }

          return (
            <>
              {healthData?.healthy ? (
                <IconCircleCheckFilled className="fill-green-400 dark:fill-green-400 px-0.5 ml-4" />

              ) : (
                <IconLoader className="animate-spin ml-4" />
              )}

            </>
          )
        },
        align: 'left' as const
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
        header: 'Restarts',
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
      {
        header: 'IP',
        accessor: (pod: Pod) => pod.status?.podIP || '-',
        cell: (value: unknown) => (
          <span className="text-sm text-muted-foreground font-mono">
            {value as string}
          </span>
        ),
      },
      ...(props.hiddenNode
        ? []
        : [
          {
            header: 'Node',
            accessor: (pod: Pod) => pod.spec?.nodeName || '-',
            cell: (value: unknown) => (
              allowLink ? (
                <Link
                  to={`/nodes/${value}`}
                  className="text-blue-600 hover:text-blue-800 hover:underline"
                >
                  {value as string}
                </Link>
              ) : (
                <span className="text-sm text-muted-foreground font-mono">
                  {value as string}
                </span>
              )
            ),
          },
        ]
      ),
      {
        header: 'Age',
        accessor: (pod: Pod) => pod.metadata?.creationTimestamp || '',
        cell: (value: unknown) => {
          const { formatted, distance } = formatDate2(value as string, true)
          return distance ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="text-muted-foreground text-sm cursor-help">
                  {distance}
                </span>
              </TooltipTrigger>
              <TooltipContent>{formatted}</TooltipContent>
            </Tooltip>
          ) : (
            <span className="text-muted-foreground text-sm">{formatted}</span>
          )
        },
      },
      {
        header: 'Actions',
        accessor: (pod: Pod) => pod || '',
        cell: (value: unknown) => {
          const pod = value as Pod
          return (
            <Button variant="destructive" size="sm"
              onClick={() => {
                setSelectedPodName(pod.metadata?.name || '')
                setSelectedPodNamespace(pod.metadata?.namespace || '')
                setIsDeleteDialogOpen(true)
              }}
            >
              <IconTrash className="w-4 h-4" />
            </Button>
          )
        },
      },
    ],
    [props.hiddenNode, props.health]
  )

  const podColumnsFiltered = podColumns
    .filter(col =>
      !(!isNested && ["Raft Role", "Healthy", "Version", "Actions"].includes(col.header))
    );
  // .filter(col =>
  //   !(isNested && ["IP", "Node"].includes(col.header))
  // );


  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <IconLoader className="animate-spin mr-2" />
        Loading pods...
      </div>
    )
  }
  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Pods</CardTitle>
        </CardHeader>
        <CardContent>
          <SimpleTable
            data={pods || []}
            columns={podColumnsFiltered}
            emptyMessage="No pods found"
            pagination={{
              enabled: true,
              pageSize: 20,
              showPageInfo: true,
            }}
          />
        </CardContent>
      </Card>

      <ResourceDeleteConfirmationDialog
        open={isDeleteDialogOpen}
        onOpenChange={setIsDeleteDialogOpen}
        resourceName={selectedPodName || ''}
        resourceType="pods"
        namespace={selectedPodNamespace || ''}
        navigateBack={isNested}
      />
    </>

  )
}
