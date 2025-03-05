import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/organisations/$organisationId/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <h2 className='text-3xl font-bold p-4'>Organisation Page</h2>
}
