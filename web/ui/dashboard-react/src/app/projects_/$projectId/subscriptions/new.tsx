import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/projects_/$projectId/subscriptions/new')(
  {
    component: CreateSubscriptionPage,
  },
)

function CreateSubscriptionPage() {
  return <div>Create a subscription!</div>
}
