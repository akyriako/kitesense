import {
  IconCircleCheckFilled,
  IconCircleXFilled,
  IconExclamationCircle,
  IconLoader,
  IconPlayerPause,
  IconTrash,
  IconTrendingDown,
  IconTrendingUp,
  IconGhost3,
  IconRotateDot,
  IconCircleDashedPlus,
  IconCircleDashed,
  IconAlertHexagonFilled,
  IconAlertCircleFilled,
  IconCircleDotFilled,
} from '@tabler/icons-react'

interface TypesenseClusterStatusIconProps {
  status: string
  className?: string
  showAnimation?: boolean
}

export const TypesenseClusterStatusIcon = ({
  status,
  className = '',
  showAnimation = true,
}: TypesenseClusterStatusIconProps) => {
  const animationClass = showAnimation ? 'animate-spin' : ''

  switch (status) {
    case 'Bootstrapping':
      return (
        <IconCircleDashedPlus
          className={`${animationClass} fill-red-500 dark:fill-red-400 ${className}`}
        />
      )

    case 'QuorumStateUnknown':
      return (
        <IconCircleDashed
          className={`fill-red-500 dark:fill-red-400 ${className}`}
        />
      )

    case 'QuorumReady':
      return (
        <IconCircleCheckFilled
          className={`fill-green-500 dark:fill-green-400 ${className}`}
        />
      )

    case 'QuorumNotReady':
      return (
        <IconAlertCircleFilled
          className={`text-red-500 dark:text-red-400 ${className}`}
        />
      )

    case 'QuorumNotReadyWaitATerm':
      return (
        <IconLoader
          className={`${animationClass} fill-blue-500 dark:fill-blue-400 ${className}`}
        />
      )

    case 'QuorumDowngraded':
      return (
        <IconTrendingDown
          className={`text-gray-500 dark:text-gray-400 ${className}`}
        />
      )

    case 'QuorumUpgraded':
      return (
        <IconTrendingUp
          className={`text-blue-500 dark:text-blue-400 ${className}`}
        />
      )  
    
    case 'QuorumNeedsAttentionMemoryOrDiskIssue':
      return (
        <IconAlertHexagonFilled
          className={`${animationClass} text-red-500 dark:text-red-400 ${className}`}
        />
      )  

    case 'QuorumNeedsAttentionClusterIsLagging':
      return (
        <IconAlertHexagonFilled
          className={`text-orange-500 dark:text-orange-400 ${className}`}
        />
      )  

    default:
      return (
        <IconAlertHexagonFilled
          className={`fill-red-500 dark:fill-red-400 ${className}`}
        />
      )
  }
}
