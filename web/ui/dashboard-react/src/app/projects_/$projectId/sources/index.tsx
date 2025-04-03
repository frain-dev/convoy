import { useState } from 'react';
import { createFileRoute, Link } from '@tanstack/react-router';

import { EllipsisVertical, Copy, Trash2, PencilLine } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from '@/components/ui/dialog';

import { useProjectStore } from '@/store';
import { transformSourceValueType } from '@/lib/pipes';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { getUserPermissions } from '@/services/auth.service';
import * as sourcesService from '@/services/sources.service';

import warningAnimation from '../../../../../assets/img/warning-animation.gif';
import sourcesEmptyState from '../../../../../assets/img/sources-empty-state.png';

export const Route = createFileRoute('/projects_/$projectId/sources/')({
	component: RouteComponent,
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	loader: async () => {
		const perms = await getUserPermissions();
		const sources = await sourcesService.getSources();

		return {
			canManageSources: perms.includes('Sources|MANAGE'),
			sources: sources,
		};
	},
});

function RouteComponent() {
	const { project } = useProjectStore();
	const { projectId } = Route.useParams();
	const { canManageSources, sources } = Route.useLoaderData();
	const [loadedSources, setLoadedSources] = useState(sources);
	const [isDeletingSource, setIsDeletingSource] = useState(false);

	async function deleteSource(sourceId: string) {
		try {
			setIsDeletingSource(true);
			await sourcesService.deleteSource(sourceId);
			const res = await sourcesService.getSources();
			setLoadedSources(res);
		} catch (error) {
			console.error(error);
			console.log('Unable to delete source');
		} finally {
			setIsDeletingSource(false);
		}
	}

	if (loadedSources.content.length === 0) {
		return (
			<DashboardLayout showSidebar={true}>
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center">
						<img
							src={sourcesEmptyState}
							alt="No subscriptions created"
							className="h-40 mb-6"
						/>
						<h2 className="font-bold mb-4 text-base text-neutral-12 text-center">
							Create your first source
						</h2>

						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							Sources are how your webhook events are routed into the Convoy.
						</p>

						<Button
							className="mt-9 mb-9 hover:bg-new.primary-400 bg-new.primary-400 text-white-100 hover:text-white-100 px-5 py-3 text-xs"
							disabled={!canManageSources}
							asChild
						>
							<Link
								to="/projects/$projectId/sources/new"
								params={{ projectId }}
							>
								<svg
									width="22"
									height="22"
									className="scale-100"
									fill="#ffffff"
								>
									<use xlinkHref="#plus-icon"></use>
								</svg>
								Connect a source
							</Link>
						</Button>
					</div>
				</div>
			</DashboardLayout>
		);
	}

	return (
		<DashboardLayout showSidebar={true}>
			<div className="mx-auto p-6 space-y-5 w-[80vw]">
				<h1 className="text-lg font-bold text-neutral-12">Sources</h1>
				<div className="grid grid-cols-2 xl:grid-cols-3 gap-6">
					<div className="rounded-[8px] border border-dashed border-new.primary-400 flex items-center justify-center hover:shadow-[0_4px_20px_-2px_rgba(50,50,71,0.08)]">
						<Link
							activeProps={{}}
							className="h-full w-full text-center py-8 flex items-center justify-center"
							to="/projects/$projectId/sources/new"
							params={{ projectId }}
							disabled={!canManageSources}
						>
							<svg
								width="22"
								height="22"
								className="mr-2 scale-75"
								fill="#477db3"
							>
								<use xlinkHref="#plus-icon"></use>
							</svg>
							<span className="text-new.primary-400 font-medium text-xs">
								Create new source
							</span>
						</Link>
					</div>

					{loadedSources.content.map(source => {
						return project?.type == 'incoming' ? (
							<div
								className="py-5 h-fit bg-white-100 rounded-[8px] border border-neutral-4 flex flex-col"
								key={source.uid}
							>
								<div className="flex flex-col px-5">
									<p className="text-[10px] text-neutral-10">
										{source.provider ||
											transformSourceValueType(
												source.verifier.type,
												'verifier',
											)}
									</p>
									<div className="flex justify-between items-center pb-2">
										<p className="text-sm text-neutral-12">{source.name}</p>
										<Dialog>
											<DropdownMenu>
												<DropdownMenuTrigger asChild>
													<Button
														variant="ghost"
														size="icon"
														className="hover:bg-transparent focus-visible:ring-0"
													>
														<EllipsisVertical className="fill-neutral-10" />
													</Button>
												</DropdownMenuTrigger>
												<DropdownMenuContent>
													<DropdownMenuItem
														className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer"
														onClick={() =>
															navigator.clipboard
																.writeText(`${source.uid}`)
																.then()
														}
													>
														<Copy className="stroke-neutral-9 !w-3 !h-3" />
														<span className="text-xs text-neutral-9">
															Copy ID
														</span>
													</DropdownMenuItem>
													<DropdownMenuItem className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer" disabled={!canManageSources}>
														<PencilLine className="stroke-neutral-9 !w-3 !h-3" />
														<span className="text-xs text-neutral-9">Edit</span>
													</DropdownMenuItem>
													<DialogTrigger asChild>
														<DropdownMenuItem className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer" disabled={!canManageSources}>
															<Trash2 className="stroke-destructive !w-3 !h-3" />
															<span className="text-xs text-destructive">
																Delete
															</span>
														</DropdownMenuItem>
													</DialogTrigger>
												</DropdownMenuContent>
											</DropdownMenu>

											<DialogContent className="sm:max-w-[432px] rounded-lg">
												<DialogHeader>
													<DialogTitle className="flex justify-center items-center">
														<img
															src={warningAnimation}
															alt="warning"
															className="w-24"
														/>
													</DialogTitle>
													<DialogDescription className="flex justify-center items-center font-medium text-new.black text-sm">
														Are you sure you want to delete this &quot;
														{source.name}&quot;?
													</DialogDescription>
												</DialogHeader>
												<div className="flex flex-col items-center space-y-4">
													<p className="text-xs text-neutral-11">
														This action is irreversible.
													</p>
													<DialogClose asChild>
														<Button
															onClick={async () =>
																await deleteSource(source.uid)
															}
															disabled={isDeletingSource || !canManageSources}
															type="submit"
															size="sm"
															className="bg-destructive text-white-100 hover:bg-destructive hover:text-white-100 focus-visible:ring-0"
														>
															Yes. Delete.
														</Button>
													</DialogClose>
												</div>
												<DialogFooter className="flex flex-row sm:justify-center items-center">
													<DialogClose
														asChild
														className="flex justify-center items-center flex-row"
													>
														<Button
															type="button"
															variant="ghost"
															className="bg-transparent hover:bg-transparent text-xs text-neutral-11 hover:text-neutral-11 font-semibold"
														>
															No. Cancel
														</Button>
													</DialogClose>
												</DialogFooter>
											</DialogContent>
										</Dialog>
									</div>
								</div>
								<hr />
								<div className="mt-5 px-5">
									<div
										className="flex items-center rounded-[22px] overflow-hidden bg-new.primary-25 gap-x-2 px-2 w-fit cursor-pointer"
										onClick={() =>
											navigator.clipboard.writeText(`${source.url}`).then()
										}
									>
										<p className="text-ellipsis overflow-hidden whitespace-nowrap text-xs">
											{source.url}
										</p>
										<Button
											size="icon"
											className="rounded-full hover:bg-transparent"
											variant={'ghost'}
											onClick={() =>
												navigator.clipboard.writeText(`${source.url}`).then()
											}
										>
											<Copy />
										</Button>
									</div>
								</div>
							</div>
						) : (
							<div
								className="w-[440px] py-5 h-fit bg-white-100 rounded-[8px] border border-neutral-4 flex flex-col"
								key={source.uid}
							>
								{source.name}
							</div>
						);
					})}
				</div>
			</div>
		</DashboardLayout>
	);
}
