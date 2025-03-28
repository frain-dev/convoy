import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/projects_/$projectId/sources/new')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Create New Source</div>
}
