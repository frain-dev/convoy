import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute(
  '/projects_/$projectId/events/event-deliveries/$eventId',
)({
  component: RouteComponent,
})

function RouteComponent() {
  return (
    <div>Hello "/projects_/$projectId/events/event-deliveries/$eventId"!</div>
  )
}
