import { DashboardSidebar } from '@/components/dashboard-sidebar';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { DashboardHeader } from '../dashboard-header';

const dashboardHeaderWidth = '1440px';
const dashboardHeaderTotalWidth = '1488px';

export function Dashboard(/* props: { children: ReactNode } */) {
	return (
		<div className="[--header-height:calc(theme(spacing.14))]">
			<SidebarProvider className="flex flex-col">
				<DashboardHeader />
				<div className="@container">
					<div className={`flex max-w-[${dashboardHeaderTotalWidth}] mx-auto`}>
						<DashboardSidebar />
						<SidebarInset
							className={`max-w-[${dashboardHeaderWidth}] p-2 mx-auto`}
						>
							{/* <div className={`max-w-[${dashboardHeaderTotalWidth}] mx-auto`}> */}
							{/* {props.children} */}
							<div className="flex flex-col gap-2">
								<div className="grid auto-rows-min gap-4 md:grid-cols-3">
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
								</div>
								<div className="min-h-[50vh] flex-1 rounded-xl bg-muted/50 @[1488px]:bg-primary-400 h-96" />
								<div className="grid auto-rows-min gap-4 md:grid-cols-3">
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
									<div className="aspect-video rounded-xl bg-muted/50 @[1488px]:bg-primary-400" />
								</div>
								<p className="text-center text-neutral-4">Convoy Version</p>
							</div>
							{/* </div> */}
						</SidebarInset>
					</div>
				</div>
			</SidebarProvider>
		</div>
	);
}
