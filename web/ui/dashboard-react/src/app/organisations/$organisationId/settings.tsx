import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/organisations/$organisationId/settings')(
  {
    component: RouteComponent,
  },
)

function RouteComponent() {
  return <h2 className='text-3xl font-bold'>Organisation Settings</h2>
}
