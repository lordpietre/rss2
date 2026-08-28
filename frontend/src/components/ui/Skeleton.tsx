export function SkeletonNewsCard() {
  return (
    <div className="card animate-pulse">
      <div className="w-full h-48 bg-gray-200 rounded-t-xl"></div>
      <div className="p-4 space-y-3">
        <div className="flex items-center gap-2">
          <div className="h-4 w-24 bg-gray-200 rounded"></div>
          <div className="h-4 w-16 bg-gray-200 rounded"></div>
        </div>
        <div className="h-5 w-3/4 bg-gray-200 rounded"></div>
        <div className="h-4 w-full bg-gray-200 rounded"></div>
        <div className="h-4 w-5/6 bg-gray-200 rounded"></div>
        <div className="h-4 w-1/2 bg-gray-200 rounded"></div>
      </div>
    </div>
  )
}

interface SkeletonNewsListProps {
  count?: number
}

export function SkeletonNewsList({ count = 6 }: SkeletonNewsListProps) {
  return (
    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }).map((_, i) => (
        <SkeletonNewsCard key={i} />
      ))}
    </div>
  )
}

export function SkeletonFeedCard() {
  return (
    <div className="card p-4 animate-pulse">
      <div className="flex items-center gap-4">
        <div className="h-10 w-10 bg-gray-200 rounded-lg"></div>
        <div className="flex-1 space-y-2">
          <div className="h-5 w-1/3 bg-gray-200 rounded"></div>
          <div className="h-4 w-1/2 bg-gray-200 rounded"></div>
        </div>
        <div className="h-6 w-20 bg-gray-200 rounded"></div>
      </div>
    </div>
  )
}

interface SkeletonFeedListProps {
  count?: number
}

export function SkeletonFeedList({ count = 5 }: SkeletonFeedListProps) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <SkeletonFeedCard key={i} />
      ))}
    </div>
  )
}

export function SkeletonEntityCard() {
  return (
    <div className="card p-4 animate-pulse">
      <div className="flex items-center gap-3">
        <div className="h-12 w-12 bg-gray-200 rounded-full"></div>
        <div className="flex-1 space-y-2">
          <div className="h-5 w-1/3 bg-gray-200 rounded"></div>
          <div className="h-3 w-1/4 bg-gray-200 rounded"></div>
        </div>
      </div>
    </div>
  )
}

interface SkeletonEntityListProps {
  count?: number
}

export function SkeletonEntityList({ count = 4 }: SkeletonEntityListProps) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <SkeletonEntityCard key={i} />
      ))}
    </div>
  )
}

export function SkeletonStats() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="card p-6 animate-pulse">
          <div className="h-4 w-1/3 bg-gray-200 rounded mb-2"></div>
          <div className="h-8 w-1/2 bg-gray-200 rounded"></div>
        </div>
      ))}
    </div>
  )
}

export function SkeletonChart() {
  return (
    <div className="card p-6 animate-pulse">
      <div className="h-4 w-1/4 bg-gray-200 rounded mb-4"></div>
      <div className="h-64 bg-gray-100 rounded"></div>
    </div>
  )
}