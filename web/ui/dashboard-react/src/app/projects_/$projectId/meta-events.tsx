import { createFileRoute, Link } from '@tanstack/react-router';
import { useState } from 'react';
import { vs } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';

import { ChevronRightIcon, RefreshCcwDot } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from '@/components/ui/table';

import { useProjectStore } from '@/store';
import { groupItemsByDate } from '@/lib/pipes';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as metaEventsService from '@/services/meta-events.service';

import detailsEmptyState from '../../../../assets/svg/empty-state.svg';
import eventsEmptyState from '../../../../assets/svg/events-empty-state-image.svg';

// Define types for the meta events
interface MetaEvent {
	uid: string;
	status: string;
	event_type: string;
	metadata: {
		num_trials: number;
		data: string;
	};
	created_at: string;
	attempt: {
		request_http_header: string;
		response_http_header: string;
	};
}

// interface MetaEventsPagination {
// 	has_next_page: boolean;
// 	has_prev_page: boolean;
// 	page: number;
// 	per_page: number;
// 	total: number;
// }

// interface MetaEventsResponse {
// 	content: MetaEvent[];
// 	pagination: MetaEventsPagination;
// }

// Placeholder component for Pagination
// NOTE: You'll need to create this component or use a pagination library
// const Pagination = ({
// 	currentPage,
// 	totalPages,
// 	onPageChange,
// }: {
// 	currentPage: number;
// 	totalPages: number;
// 	onPageChange: (page: number) => void;
// }) => (
// 	<div className="flex justify-center items-center gap-2 mt-4">
// 		<Button
// 			variant="outline"
// 			size="sm"
// 			onClick={() => onPageChange(currentPage - 1)}
// 			disabled={currentPage <= 1}
// 		>
// 			Previous
// 		</Button>
// 		<span>
// 			Page {currentPage} of {totalPages}
// 		</span>
// 		<Button
// 			variant="outline"
// 			size="sm"
// 			onClick={() => onPageChange(currentPage + 1)}
// 			disabled={currentPage >= totalPages}
// 		>
// 			Next
// 		</Button>
// 	</div>
// );

// Placeholder component for EmptyState
// NOTE: You'll need to create this component
const EmptyState = ({
	className,
	image,
	description,
}: {
	className?: string;
	image: string;
	description: string;
}) => (
	<div className={`flex flex-col items-center justify-center ${className}`}>
		<img src={image} alt="Empty state" className="h-24 mb-4" />
		<p className="text-neutral-10 text-sm">{description}</p>
	</div>
);

export const Route = createFileRoute('/projects_/$projectId/meta-events')({
	component: MetaEventsPage,
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	async loader() {
		const metaEvents = await metaEventsService.getMetaEvents();

		return { metaEvents };
	},
});

function MetaEventsPage() {
	const { project } = useProjectStore();
	const { projectId } = Route.useParams();
	const { metaEvents } = Route.useLoaderData();
	const isMetaEventEnabled = project?.config.meta_event.is_enabled;
	const [displayedMetaEvents, setDisplayedMetaEvents] = useState<MetaEvent[]>(
		metaEvents.content,
	);
	const [selectedMetaEvent, setSelectedMetaEvent] = useState<MetaEvent | null>(
		null,
	);
	const [isRetryingMetaEvent, setIsRetryingMetaEvent] = useState(false);

	const metaEventsTableHead = [
		'Status',
		'Event Types',
		'Retries',
		'Time',
		'',
		'',
	];

	function setTagColour(status: string) {
		switch (status) {
			case 'Success':
				return 'bg-new.success-25 text-new.success-600 hover:bg-new.success-25 hover:new.success-600';
			case 'Failure':
			case 'Failed':
				return 'bg-destructive/10 text-destructive hover:bg-destructive/10 hover:text-destructive';
			// Pending
			default:
				return 'bg-neutral-3 text-neutral-10 hover:bg-neutral-3 hover:text-neutral-10';
		}
	}

	const retryMetaEvent = async (eventId: string) => {
		setIsRetryingMetaEvent(true);
		try {
			const ev = await metaEventsService.retryEvent(eventId);
			setSelectedMetaEvent(prev => ({ ...prev, ...ev }));
			setDisplayedMetaEvents(prev =>
				prev.map(_ev => (ev.uid == _ev.uid ? { ..._ev, ...ev } : _ev)),
			);
		} catch (error) {
			console.error('Error retrying meta event:', error);
		} finally {
			setIsRetryingMetaEvent(false);
		}
	};

	const getCodeSnippetString = (
		type:
			| 'event_data'
			| 'res_body'
			| 'res_header'
			| 'req_header'
			| 'error'
			| 'log',
		data: any,
	) => {
		let displayMessage = '';
		switch (type) {
			case 'event_data':
				displayMessage = 'No event payload was sent';
				break;
			case 'res_body':
				displayMessage = 'No response body was sent';
				break;
			case 'res_header':
				displayMessage = 'No response header was sent';
				break;
			case 'req_header':
				displayMessage = 'No request header was sent';
				break;
			default:
				displayMessage = '';
				break;
		}

		if (data)
			return JSON.stringify(data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		return displayMessage;
	};

	return (
		<DashboardLayout showSidebar={true}>
			{!isMetaEventEnabled && (
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center">
						<img
							src={eventsEmptyState}
							alt="No subscriptions created"
							className="mb-12 h-40"
						/>
						<h2 className="font-bold mb-4 text-base text-neutral-12 text-center">
							Turn On Meta Events to Start Seeing Events
						</h2>

						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							Meta events are operation events that occur on your project, such
							as: event delivery, endpoints status, e.t.c. You can receive this
							events notification via HTTPS.
						</p>

						<Button
							className="mt-9 mb-9 hover:bg-new.primary-400 bg-new.primary-400 text-white-100 hover:text-white-100 px-5 py-3 text-xs"
							asChild
						>
							<Link
								to="/projects/$projectId/settings"
								params={{ projectId }}
								search={{ active_tab: 'meta-events' }}
								className="py-5"
							>
								Turn on Meta Events
							</Link>
						</Button>
					</div>
				</div>
			)}

			{isMetaEventEnabled && (
				<div className="p-6">
					<section className="space-y-6 w-full max-w-[1440px]">
						<h1 className="text-lg font-bold text-neutral-12">Meta Events</h1>

						<div className="flex border rounded-8px">
							<div className="min-w-[605px] w-full h-full overflow-hidden relative">
								{/* {isLoadingMetaEvents && (
									<div className="animate-pulse py-10">
										{[1, 2, 3, 4, 5].map(index => (
											<div
												key={index}
												className="h-12 bg-neutral-3 rounded-md my-1 mx-4"
											></div>
										))}
									</div>
								)} */}

								<div
									className="min-h-[70vh] max-h-[70vh] overflow-y-auto overflow-x-auto w-full min-w-[485px]"
									id="events-table-container"
								>
									<Table>
										<TableHeader>
											<TableRow>
												{metaEventsTableHead.map((head, i) => (
													<TableHead
														key={i}
														className={`uppercase text-xs text-neutral-12 ${i === 0 ? 'pl-5' : ''}`}
													>
														{head}
													</TableHead>
												))}
											</TableRow>
										</TableHeader>
										<TableBody>
											{displayedMetaEvents.length > 0 &&
												Array.from(
													groupItemsByDate(displayedMetaEvents, 'asc'),
												).map(([dateKey, events]) => {
													return [
														<TableRow className="bg-neutral-2" key={dateKey}>
															<TableCell
																colSpan={6}
																className="py-2 px-5 font-normal text-xs text-neutral-10"
															>
																{dateKey}
															</TableCell>
														</TableRow>,
													].concat(
														events.map(ev => (
															<TableRow
																key={ev.uid}
																onClick={() => setSelectedMetaEvent(ev)}
																className={`hover:bg-new.primary-25 hover:cursor-pointer ${ev.uid == selectedMetaEvent?.uid && 'bg-new.primary-25'}`}
															>
																<TableCell className="w-32 pl-4 pr-8 relative">
																	<Badge
																		className={`shadow-none font-normal text-xs border-0 !rounded-22px py-1.5 px-3 ${setTagColour(ev.status)}`}
																	>
																		{ev.status}
																	</Badge>
																</TableCell>
																<TableCell>
																	<Badge className="shadow-none font-normal text-xs border-0 !rounded-22px py-1.5 px-3 bg-neutral-a3 hover:bg-neutral-a3 text-neutral-11">
																		{ev.event_type}
																	</Badge>
																</TableCell>
																<TableCell className="text-xs font-normal">
																	{ev.metadata.num_trials}
																</TableCell>
																<TableCell className="uppercase text-xs font-normal">
																	{new Intl.DateTimeFormat('en-GB', {
																		hour: 'numeric',
																		minute: 'numeric',
																		second: 'numeric',
																		hour12: true,
																	}).format(new Date(ev.created_at))}
																</TableCell>
																<TableCell>
																	<Button
																		variant="ghost"
																		size="sm"
																		disabled={isRetryingMetaEvent}
																		onClick={e => {
																			e.stopPropagation();
																			retryMetaEvent(ev.uid);
																		}}
																		className="flex items-center gap-2 py-1 shadow-none text-xs text-new.primary-400 hover:text-xs hover:text-new.primary-400 hover:bg-transparent"
																	>
																		<RefreshCcwDot className="stroke-new.primary-400" />
																		Retry
																	</Button>
																</TableCell>
																<TableCell>
																	<Button
																		variant="ghost"
																		size="icon"
																		className="hover:bg-transparent"
																	>
																		<ChevronRightIcon className="stroke-new.primary-400" />
																	</Button>
																</TableCell>
															</TableRow>
														)),
													);
												})}

											{displayedMetaEvents.length === 0 && (
												<TableRow>
													<TableCell colSpan={6}>
														<div className="mx-auto w-full">
															<EmptyState
																className="my-36"
																image={detailsEmptyState}
																description="No meta event has been sent"
															/>
														</div>
													</TableCell>
												</TableRow>
											)}
										</TableBody>
									</Table>
								</div>
							</div>

							<div className="max-w-[450px] w-full max-h-[calc(100vh - 950px)] min-h-[707px] overflow-auto relative border-l">
								<div className="p-4">
									{selectedMetaEvent && (
										<>
											<h3 className="text-base font-bold mb-4">Details</h3>
											<div className="mb-5">
												<h4 className="text-sm font-medium mb-2">
													Request Header
												</h4>
												<SyntaxHighlighter
													language="json"
													style={vs}
													showLineNumbers={true}
													className="rounded-md text-sm"
												>
													{getCodeSnippetString(
														'req_header',
														selectedMetaEvent?.attempt?.request_http_header,
													)}
												</SyntaxHighlighter>
											</div>
											<div className="mb-5">
												<h4 className="text-sm font-medium mb-2">
													Response Header
												</h4>
												<SyntaxHighlighter
													language="json"
													showLineNumbers={true}
													style={vs}
													className="rounded-md text-sm"
												>
													{getCodeSnippetString(
														'res_header',
														selectedMetaEvent?.attempt?.response_http_header,
													)}
												</SyntaxHighlighter>
											</div>
											<div className="mb-5">
												<h4 className="text-sm font-medium mb-2">
													Response Body
												</h4>
												<SyntaxHighlighter
													language="json"
													showLineNumbers={true}
													style={vs}
													className="rounded-md text-sm"
													// lineNumberStyle={{ fontStyle: 'normal', marginLeft: 0, paddingLeft: 0 }}
												>
													{getCodeSnippetString(
														'res_body',
														selectedMetaEvent?.metadata?.data,
													)}
												</SyntaxHighlighter>
											</div>
										</>
									)}

									{!selectedMetaEvent && (
										<EmptyState
											className="my-36"
											image={detailsEmptyState}
											description={
												!selectedMetaEvent && displayedMetaEvents.length == 0
													? 'No meta event has been sent'
													: 'Click on a meta event to display its details'
											}
										/>
									)}
								</div>
							</div>
						</div>

						{/* Pagination */}
						{/* {metaEvents?.pagination?.has_next_page ||
							(metaEvents?.pagination?.has_prev_page && (
								<Pagination
									currentPage={metaEvents.pagination.page}
									totalPages={Math.ceil(
										metaEvents.pagination.total /
											metaEvents.pagination.per_page,
									)}
									onPageChange={getMetaEvents}
								/>
							))} */}
					</section>
				</div>
			)}
		</DashboardLayout>
	);
}
