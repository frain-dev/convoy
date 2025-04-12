import React, { useState, useEffect, useRef } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { format, setHours, setMinutes } from 'date-fns';
import { formatInTimeZone } from 'date-fns-tz';
import { vs } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';

import { createFileRoute } from '@tanstack/react-router';

import {
	ChevronDown,
	Check,
	CalendarIcon,
	Copy,
	ArrowUpRight,
	RefreshCw,
	ArrowRight,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { ConvoyCheckbox } from '@/components/convoy-checkbox';
import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from '@/components/ui/select';
import {
	Popover,
	PopoverTrigger,
	PopoverContent,
	PopoverClose,
} from '@/components/ui/popover';
import {
	Command,
	CommandInput,
	CommandList,
	CommandEmpty,
	CommandGroup,
	CommandItem,
} from '@/components/ui/command';
import { Calendar } from '@/components/ui/calendar';
import { DashboardLayout } from '@/components/dashboard';
import { Badge } from '@/components/ui/badge';
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog';
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';

import { cn } from '@/lib/utils';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as eventsService from '@/services/events.service';
import * as sourcesService from '@/services/sources.service';
import * as eventLogService from '@/services/event-log.service';

import type { DateRange } from 'react-day-picker';
import type { Event, EventDelivery } from '@/models/event';

import searchIcon from '../../../../assets/svg/search-icon.svg';
import eventsLogEmptyStateImg from '../../../../assets/svg/events-empty-state-image.svg';
import { useProjectStore } from '@/store';

const EventsLogSearchSchema = z.object({
	sort: z.enum(['asc', 'desc']).catch('desc'),
	next_page_cursor: z.string().catch(''),
	direction: z.enum(['next', 'prev', '']).optional().catch(''),
	source_id: z.string().catch(''),
	startDate: z.string().optional().catch(''),
	endDate: z.string().optional().catch(''),
	query: z.string().catch(''),
});

export const Route = createFileRoute('/projects_/$projectId/events-log')({
	component: EventsLogPage,
	validateSearch: EventsLogSearchSchema,
	beforeLoad({ context, search }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
		return { search };
	},
	async loader() {
		const events = await eventsService.getEvents();
		const sources = await sourcesService.getSources();

		return { events, sources };
	},
});

function EventsLogPage() {
	const navigate = Route.useNavigate();
	const { events: initialEvents, sources } = Route.useLoaderData();
	const search = Route.useSearch();
	const { project } = useProjectStore();
	const [loadedSources, setLoadedSources] = useState(sources.content);

	// Search and filtering state
	const [searchString, setSearchString] = useState(search.query);
	const [date, setDate] = useState<DateRange | undefined>(
		!search.startDate || !search.endDate
			? undefined
			: {
					from: new Date(search.startDate),
					to: new Date(search.endDate),
				},
	);
	const [endTimeValue, setEndTimeValue] = useState('23:59');
	const [startTimeValue, setStartTimeValue] = useState('00:00');

	// Events and event details state
	const [_events, setEvents] = useState(initialEvents);
	const [displayedEvents, setDisplayedEvents] = useState<
		Array<{ date: string; content: Array<Event> }>
	>([]);
	const [isLoadingEvents, setIsLoadingEvents] = useState(false);
	const [eventsDetailsItem, setEventsDetailsItem] = useState<Event | null>(
		null,
	);
	const [sidebarEventDeliveries, setSidebarEventDeliveries] = useState<
		Array<EventDelivery>
	>([]);
	const [isLoadingSidebarDeliveries, setIsLoadingSidebarDeliveries] =
		useState(false);
	const [enableTailMode, setEnableTailMode] = useState(false);
	const [duplicateEvents, setDuplicateEvents] = useState<Array<Event>>([]);
	const [isFetchingDuplicateEvents, setIsFetchingDuplicateEvents] =
		useState(false);
	const [isRetrying, setIsRetrying] = useState(false);
	const [batchRetryCount, setBatchRetryCount] = useState(0);
	const [showBatchRetryDialog, setShowBatchRetryDialog] = useState(false);

	const eventLogsTableHead = ['Event ID', 'Source', 'Time', ''];
	const eventsInterval = useRef<number | null>(null);

	const filterForm = useForm({
		resolver: zodResolver(
			z.object({
				source_id: z.string().optional(),
			}),
		),
		defaultValues: {
			source_id: search.source_id || '',
		},
	});

	// Format code for JSON display
	const formatCode = (code: unknown): string => {
		return typeof code === 'string' ? code : JSON.stringify(code, null, 2);
	};

	// Copy text to clipboard
	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text);
		// Could add a notification here
	};

	function setTimeOnDate(time: string, date: Date) {
		const [hours, minutes] = time.split(':').map(str => parseInt(str, 10));
		return setHours(setMinutes(new Date(date), minutes), hours);
	}

	// Get event deliveries for sidebar
	const getEventDeliveriesForSidebar = async (eventId: string) => {
		setIsLoadingSidebarDeliveries(true);
		setSidebarEventDeliveries([]);

		try {
			const response = await eventsService.getEventDeliveries({ eventId });
			setSidebarEventDeliveries(response.content);
			setIsLoadingSidebarDeliveries(false);
		} catch (error) {
			setIsLoadingSidebarDeliveries(false);
		}
	};

	// Get duplicate events
	const getDuplicateEvents = async (event: Event) => {
		if (!event.is_duplicate_event || !event.idempotency_key) return;

		setIsFetchingDuplicateEvents(true);
		try {
			const eventsResponse = await eventsService.getEvents({
				idempotencyKey: event.idempotency_key,
			});
			setDuplicateEvents(eventsResponse.content);
			setIsFetchingDuplicateEvents(false);
		} catch (error) {
			setIsFetchingDuplicateEvents(false);
		}
	};

	// Replay event
	const replayEvent = async (requestDetails: { eventId: string }) => {
		setIsRetrying(true);
		try {
			await eventLogService.retryEvent(requestDetails.eventId);
			// Could add notification here
			setIsRetrying(false);
		} catch (error) {
			setIsRetrying(false);
		}
	};

	// Fetch retry count
	const fetchRetryCount = async (requestDetails?: Record<string, unknown>) => {
		try {
			// Create parameter object matching expected API format
			const params = {
				startDate: search.startDate
					? formatInTimeZone(search.startDate, 'UTC', "yyyy-MM-dd'T'HH:mm:ss")
					: undefined,
				endDate: search.endDate
					? formatInTimeZone(search.endDate, 'UTC', "yyyy-MM-dd'T'HH:mm:ss")
					: undefined,
				sourceId: search.source_id || undefined,
				query: search.query || undefined,
				...requestDetails,
			};

			// Filter out undefined values
			const filteredParams = Object.fromEntries(
				Object.entries(params).filter(
					([_, value]) => value !== undefined && value !== '',
				),
			);

			const response = await eventLogService.getRetryCount(filteredParams);
			setBatchRetryCount(response.count);
			setShowBatchRetryDialog(true);
		} catch (error) {
			console.error(error);
		}
	};

	// Batch replay events
	const batchReplayEvent = async () => {
		setIsRetrying(true);

		try {
			// Create parameter object matching expected API format
			const params = {
				startDate: search.startDate
					? formatInTimeZone(search.startDate, 'UTC', "yyyy-MM-dd'T'HH:mm:ss")
					: undefined,
				endDate: search.endDate
					? formatInTimeZone(search.endDate, 'UTC', "yyyy-MM-dd'T'HH:mm:ss")
					: undefined,
				sourceId: search.source_id || undefined,
				query: search.query || undefined,
			};

			// Filter out undefined values
			const filteredParams = Object.fromEntries(
				Object.entries(params).filter(
					([_, value]) => value !== undefined && value !== '',
				),
			);

			await eventLogService.batchRetryEvent(filteredParams);
			setShowBatchRetryDialog(false);
			setIsRetrying(false);
		} catch (error) {
			setIsRetrying(false);
		}
	};

	// Set up initial events and tailing
	useEffect(() => {
		// Fetch event logs
		const fetchEventLogs = async (requestDetails?: Record<string, unknown>) => {
			setIsLoadingEvents(true);

			try {
				// Create a properly formatted parameters object that matches the expected API format
				const startDate = search.startDate
					? new Date(search.startDate).toISOString().split('.')[0]
					: undefined;
				const endDate = search.endDate
					? new Date(search.endDate).toISOString().split('.')[0]
					: undefined;

				const params = {
					idempotencyKey: search.query?.includes('idempotency_key:')
						? search.query.split('idempotency_key:')[1].trim()
						: undefined,
					query: search.query?.includes('idempotency_key:')
						? undefined
						: search.query,
					sourceId: search.source_id || undefined,
					sort: search.sort || 'desc',
					next_page_cursor: search.next_page_cursor,
					direction: search.direction,
					...requestDetails,
					showLoader: true,
				};

				// Filter out undefined values to keep the request clean
				const filteredParams = Object.fromEntries(
					Object.entries(params).filter(
						([_, value]) => value !== undefined && value !== '',
					),
				);

				const eventsResponse = await eventsService.getEvents({
					...filteredParams,
					startDate,
					endDate,
				});

				setEvents(eventsResponse);
				const groupedEvents = groupEventsByDate(eventsResponse.content);
				setDisplayedEvents(groupedEvents);

				if (!eventsDetailsItem && eventsResponse.content.length > 0) {
					setEventsDetailsItem(eventsResponse.content[0]);
					getEventDeliveriesForSidebar(eventsResponse.content[0].uid);
					getDuplicateEvents(eventsResponse.content[0]);
				}

				setIsLoadingEvents(false);
			} catch (error) {
				setIsLoadingEvents(false);
			}
		};

		// Handle tailing
		const handleTailing = (enabled: boolean) => {
			if (enabled && !eventsInterval.current) {
				eventsInterval.current = window.setInterval(() => {
					fetchEventLogs(search);
				}, 5000);
			} else if (!enabled && eventsInterval.current) {
				clearInterval(eventsInterval.current);
				eventsInterval.current = null;
			}
		};

		fetchEventLogs(search);

		if (enableTailMode) {
			handleTailing(true);
		}

		return () => {
			if (eventsInterval.current) {
				clearInterval(eventsInterval.current);
			}
		};
	}, [search, enableTailMode, eventsDetailsItem]);

	useEffect(() => {
		setLoadedSources(sources.content);
	}, [sources]);

	// Helper function for date grouping
	const groupEventsByDate = (events: Array<Event>) => {
		if (!events?.length) return [];

		const groups: Array<{ date: string; content: Array<Event> }> = [];

		events.forEach(event => {
			const date = new Date(event.created_at).toLocaleDateString('en-US', {
				year: 'numeric',
				month: 'long',
				day: 'numeric',
			});

			const existingGroup = groups.find(group => group.date === date);

			if (existingGroup) {
				existingGroup.content.push(event);
			} else {
				groups.push({
					date,
					content: [event],
				});
			}
		});

		return groups;
	};

	// TODO move this function to a utility
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

	return (
		<DashboardLayout showSidebar={true}>
			<div className="p-6">
				<section className="py-6 w-full max-w-[1440px]">
					<h1 className="text-lg font-bold text-neutral-12">Events Log</h1>
				</section>

				{/* Filter Section */}
				<div className="flex items-end justify-between">
					<div className="flex items-center gap-x-2">
						<div>
							<form
								className="flex flex-col gap-y-2"
								onSubmit={e => {
									e.preventDefault();
									navigate({
										to: Route.fullPath,
										search: { ...search, query: searchString },
									});
								}}
							>
								<label
									className="text-neutral-9 text-xs"
									htmlFor="search_events"
								>
									Search events
								</label>
								<div className="border border-primary-400 h-9 px-[14px] py-0 max-w-60 w-full rounded-[10px] flex items-center bg-white-100">
									<img src={searchIcon} alt="search icon" className="mr-2" />
									<input
										type="search"
										id="search_events"
										className="w-full text-neutral-11 text-xs outline-none"
										value={searchString}
										onChange={e => setSearchString(e.target.value)}
									/>
									{searchString && (
										<Button
											type="submit"
											variant="ghost"
											size="sm"
											className="transition-all duration-200 hover:bg-transparent"
										>
											<img
												src={searchIcon}
												alt="enter icon"
												className="w-[16px]"
											/>
										</Button>
									)}
								</div>
							</form>
						</div>
						<div className="flex flex-col gap-y-2">
							<label
								htmlFor="sort_events_log"
								className="text-neutral-9 text-xs"
							>
								Sort
							</label>
							<Select
								defaultValue={search.sort || 'desc'}
								onValueChange={sort =>
									navigate({
										to: Route.fullPath,
										search: { ...search, sort },
									})
								}
							>
								<SelectTrigger
									className="w-[80px] h-9 shadow-none focus:ring-0 text-xs text-neutral-9"
									id="sort_events_log"
								>
									<SelectValue
										className="text-xs text-neutral-9"
										placeholder="Sort"
									/>
								</SelectTrigger>
								<SelectContent>
									<SelectItem
										className="text-xs hover:cursor-pointer"
										value="asc"
									>
										<span className="text-neutral-9">Asc</span>
									</SelectItem>
									<SelectItem
										className="text-xs hover:cursor-pointer"
										value="desc"
									>
										<span className="text-neutral-9">Desc</span>
									</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div>
							<div className="px-2 py-1 border border-neutral-5 rounded-8px mt-6">
								<ConvoyCheckbox
									isChecked={enableTailMode}
									label="Tail Events"
									onChange={e => setEnableTailMode(e.target.checked)}
								/>
							</div>
						</div>
						<div className="flex flex-col gap-y-2">
							<label className="text-neutral-9 text-xs" htmlFor="date">
								Select a date range
							</label>
							<div className="grid gap-2">
								<Popover>
									<PopoverTrigger asChild className="shadow-none">
										<Button
											id="date"
											variant={'outline'}
											className={cn(
												'w-[320px] justify-start text-left font-normal text-xs text-neutral-9',
											)}
										>
											<CalendarIcon className="stroke-neutral-9" />
											{date?.from ? (
												date.to ? (
													<>
														{formatInTimeZone(
															date.from,
															'UTC',
															'dd/LL/y, h:mm aa',
														)}{' '}
														-{' '}
														{formatInTimeZone(
															date.to,
															'UTC',
															'dd/LL/y, h:mm aa',
														)}
													</>
												) : (
													format(date.from, 'dd/LL/y, h:mm aa')
												)
											) : (
												<span className="text-neutral-9">
													Pick a date range
												</span>
											)}
										</Button>
									</PopoverTrigger>
									<PopoverContent className="w-auto p-0" align="start">
										<Calendar
											initialFocus
											mode="range"
											defaultMonth={date?.from}
											selected={date}
											onSelect={setDate}
											numberOfMonths={2}
										/>
										<form
											className="px-4 flex gap-x-6 items-center"
											style={{ marginBlockEnd: '1em' }}
										>
											<div className="">
												<label className="text-sm">
													Start time:{' '}
													<input
														type="time"
														value={startTimeValue}
														onChange={e => {
															const time = e.target.value;
															setStartTimeValue(time);
														}}
													/>
												</label>
											</div>
											<div>
												<label className="text-sm">
													End time:{' '}
													<input
														type="time"
														value={endTimeValue}
														onChange={e => {
															const time = e.target.value;
															setEndTimeValue(time);
														}}
													/>
												</label>
											</div>
										</form>
										<div className="px-4 pb-2 flex justify-between gap-x-4 items-center">
											<span className="text-xs text-neutral-9 ">
												Time would be converted to UTC after selection
											</span>
											<div className="flex items-center gap-x-2">
												<PopoverClose asChild>
													<Button
														variant={'ghost'}
														size={'sm'}
														className="border border-destructive bg-transparent hover:bg-transparent hover:text-destructive text-destructive text-xs"
														onClick={() => {
															setDate(undefined);
															navigate({
																to: Route.fullPath,
																search: {
																	...search,
																	startDate: '',
																	endDate: '',
																},
															});
														}}
													>
														Clear
													</Button>
												</PopoverClose>
												<PopoverClose asChild>
													<Button
														variant={'ghost'}
														size={'sm'}
														className="bg-new.primary-400 text-white-100 hover:text-white-100 hover:bg-new.primary-400 text-xs"
														onClick={() => {
															if (!date?.from || !date?.to) return;

															const from = setTimeOnDate(
																startTimeValue,
																date.from,
															);
															const to = setTimeOnDate(endTimeValue, date.to);
															setDate({ from, to });
															navigate({
																to: Route.fullPath,
																search: {
																	...search,
																	startDate: from?.toUTCString(),
																	endDate: to?.toUTCString(),
																},
															});
														}}
													>
														Apply
													</Button>
												</PopoverClose>
											</div>
										</div>
									</PopoverContent>
								</Popover>
							</div>
						</div>

						<div>
							<Form {...filterForm}>
								<form>
									<FormField
										control={filterForm.control}
										name="source_id"
										render={({ field }) => (
											<FormItem className="flex flex-col gap-y-2">
												<FormLabel className="text-neutral-9 text-xs">
													Select source
												</FormLabel>
												<Popover>
													<PopoverTrigger asChild className="shadow-none w-60">
														<FormControl>
															<Button
																variant="outline"
																role="combobox"
																className="flex items-center text-xs text-neutral-10 hover:text-neutral-10"
															>
																{field.value
																	? [{ name: 'All', uid: '' }]
																			.concat(loadedSources)
																			.find(
																				source => source.uid === field.value,
																			)?.name
																	: 'All'}
																<ChevronDown className="ml-auto opacity-50" />
															</Button>
														</FormControl>
													</PopoverTrigger>
													<PopoverContent
														align="start"
														className="p-0 shadow-none"
													>
														<Command className="shadow-none">
															<CommandInput
																placeholder="Search sources"
																className="h-9 placeholder:text-xs text-xs"
															/>
															<CommandList className="max-h-40">
																<CommandEmpty className="text-xs text-neutral-10 hover:text-neutral-10 py-4">
																	No sources found.
																</CommandEmpty>
																<CommandGroup>
																	{[{ name: 'All', uid: '' }]
																		.concat(loadedSources)
																		.map(source => (
																			<PopoverClose
																				key={source.uid}
																				className="flex flex-col w-full"
																			>
																				<CommandItem
																					className="cursor-pointer text-xs !text-neutral-10 py-2 !hover:text-neutral-10"
																					value={`${source.name}-${source.uid}`}
																					onSelect={() => {
																						field.onChange(source.uid);
																						navigate({
																							to: Route.fullPath,
																							search: {
																								...search,
																								source_id: source.uid,
																							},
																						});
																					}}
																				>
																					{source.name}
																					<Check
																						className={cn(
																							'ml-auto',
																							source.uid === field.value
																								? 'opacity-100 stroke-neutral-10'
																								: 'opacity-0',
																						)}
																					/>
																				</CommandItem>
																			</PopoverClose>
																		))}
																</CommandGroup>
															</CommandList>
														</Command>
													</PopoverContent>
												</Popover>
												<FormMessageWithErrorIcon />
											</FormItem>
										)}
									/>
								</form>
							</Form>
						</div>
					</div>
					<div>
						<Button
							size="sm"
							variant="ghost"
							className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
							disabled={batchRetryCount === 0}
							onClick={() => fetchRetryCount(search)}
						>
							Batch Retry
						</Button>
					</div>
				</div>
			</div>

			{/* Empty state or Main content section */}
			{!isLoadingEvents &&
			(!displayedEvents || displayedEvents.length === 0) ? (
				<div className="m-auto py-80px">
					<div className="flex flex-col items-center justify-center">
						<img
							src={eventsLogEmptyStateImg}
							alt="No events"
							className="h-40 mb-6"
						/>
						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							You currently do not have any event logs
						</p>
					</div>
				</div>
			) : (
				<div className="flex gap-6 border-t border-t-new.primary-50 px-6">
					{/* Events Table */}
					<div className="w-full overflow-hidden relative">
						{isLoadingEvents ? (
							<div className="animate-pulse py-10">
								{[1, 2, 3, 4, 5].map(index => (
									<div
										key={index}
										className="h-12 bg-neutral-3 rounded-md my-1"
									></div>
								))}
							</div>
						) : (
							<div
								className="min-h-[70vh] overflow-y-auto overflow-x-auto w-full min-w-[485px]"
								id="events-table-container"
							>
								<Table>
									<TableHeader>
										<TableRow>
											{eventLogsTableHead.map((head, i) => (
												<TableHead
													key={i}
													className={`uppercase text-xs font-medium text-neutral-12 ${i === 0 ? 'pl-5' : ''}`}
												>
													{head}
												</TableHead>
											))}
										</TableRow>
									</TableHeader>
									<TableBody>
										{displayedEvents.map((eventGroup, groupIndex) => (
											<React.Fragment key={groupIndex}>
												{/* Date Row */}
												<TableRow className="border-t border-new.primary-25">
													<TableCell className="pt-4 pl-4 pb-2 text-neutral-10 text-xs">
														{eventGroup.date}
													</TableCell>
													<TableCell className="pt-4 pb-2 text-neutral-10"></TableCell>
													<TableCell className="pt-4 pb-2 text-neutral-10"></TableCell>
													<TableCell className="pt-4 pb-2 text-neutral-10"></TableCell>
												</TableRow>

												{/* Event Rows */}
												{eventGroup.content.map((event, eventIndex) => (
													<TableRow
														key={eventIndex}
														className={`cursor-pointer group hover:bg-new.primary-25 transition-all duration-300 ${event.uid === eventsDetailsItem?.uid ? 'bg-new.primary-25' : ''}`}
														onClick={() => {
															setEventsDetailsItem(event);
															getEventDeliveriesForSidebar(event.uid);
															getDuplicateEvents(event);
														}}
													>
														<TableCell className="w-[380px] pl-4 pr-8 relative rounded-l-8px py-3">
															<div className="flex items-center truncate gap-2">
																<Badge className="max-w-[260px] gap-x-4 truncate font-normal flex items-center !rounded-22px text-sm bg-neutral-a3 hover:bg-neutral-a3 px-3 py-1.5 text-neutral-11">
																	<span className="text-neutral-12 text-xs">
																		{event.uid}
																	</span>

																	<Button
																		variant="ghost"
																		size="icon"
																		className="h-4 w-4 hover:bg-transparent"
																		onClick={e => {
																			e.stopPropagation();
																			copyToClipboard(event.uid);
																		}}
																	>
																		<Copy className="h-3 w-3 stroke-neutral-9" />
																	</Button>
																</Badge>
															</div>
														</TableCell>

														<TableCell className="py-3">
															<div className="max-w-[300px] w-full overflow-hidden overflow-ellipsis text-xs font-normal text-neutral-12">
																{event.source_metadata?.name || 'Rest API'}
															</div>
														</TableCell>

														<TableCell className="py-3 text-xs font-normal text-neutral-11">
															{new Date(event.created_at).toLocaleTimeString()}
														</TableCell>

														<TableCell className="flex justify-end items-center gap-x-2 rounded-r-8px py-3">
															{event.is_duplicate_event && (
																<Badge className="bg-neutral-a3 text-neutral-11 shadow-none text-xs font-normal rounded-22px hover:bg-neutral-a3">
																	Duplicate
																</Badge>
															)}

															<Button
																variant="ghost"
																size="sm"
																className="pr-5 hover:bg-transparent"
																title="event deliveries"
																onClick={e => {
																	e.stopPropagation();
																	// Navigate to event deliveries
																	// TO DO: Implement navigation to event deliveries
																}}
															>
																<ArrowUpRight className="h-3.5 w-3.5 stroke-neutral-10" />
															</Button>
														</TableCell>
													</TableRow>
												))}
											</React.Fragment>
										))}
									</TableBody>
								</Table>
							</div>
						)}
					</div>

					{/* Sidebar Separator */}
					<div className="w-[1px] bg-new.primary-50"></div>

					{/* Sidebar for Event Details */}
					<div className="max-w-[472px] w-full max-h-[calc(100vh - 950px)] min-h-[707px] overflow-auto relative pt-4">
						{/* Sidebar loader */}
						{isLoadingEvents ? (
							<>
								<div className="border-b border-new.primary-25 pb-6">
									<Skeleton className="h-4 w-20 rounded-full" />
								</div>
								<div className="flex justify-between border-y border-new.primary-25 py-6 mb-5">
									<Skeleton className="h-4 w-20 rounded-full" />
									<Skeleton className="h-4 w-52 rounded-full" />
								</div>
							</>
						) : (
							/* Event details */
							eventsDetailsItem && (
								<>
									<div className="border-b border-new.primary-25 pb-6">
										<Button
											size="sm"
											variant="ghost"
											onClick={() =>
												replayEvent({ eventId: eventsDetailsItem.uid })
											}
											disabled={isRetrying}
											className="flex items-center shadow-none text-new.primary-400 hover:text-new.primary-400 hover:bg-new.primary-25 bg-new.primary-25 px-2 py-1"
										>
											<RefreshCw className="h-4 w-4 stroke-new.primary-400" />
											Replay
										</Button>
									</div>

									<div className="flex items-center border-b border-new.primary-25 py-6 text-xs">
										<p className="text-neutral-10 mr-6">Idempotency Key</p>
										<p className="text-neutral-10 w-[280px] overflow-hidden overflow-ellipsis">
											{eventsDetailsItem.idempotency_key || '-'}
										</p>
									</div>
								</>
							)
						)}

						{/* Duplicate events */}
						{eventsDetailsItem?.is_duplicate_event && (
							<>
								{isFetchingDuplicateEvents ? (
									<div className="mt-5">
										<Skeleton className="h-4 w-20 rounded-full mb-8" />
										<div className="flex justify-between mb-5">
											<Skeleton className="h-4 w-20 rounded-full" />
											<Skeleton className="h-4 w-52 rounded-full" />
											<Skeleton className="h-4 w-16 rounded-full" />
										</div>
										<div className="flex justify-between mb-5">
											<Skeleton className="h-4 w-20 rounded-full" />
											<Skeleton className="h-4 w-52 rounded-full" />
											<Skeleton className="h-4 w-16 rounded-full" />
										</div>
									</div>
								) : (
									duplicateEvents?.length > 0 && (
										<>
											<p className="text-xs text-neutral-10 font-medium my-4">
												Duplicate Events
											</p>
											<ul className="border-b border-new.primary-25 mb-4">
												{duplicateEvents.map(event => (
													<li
														key={event.uid}
														className="cursor-pointer border-none flex mb-2.5 hover:bg-new.primary-25 py-1.5 rounded-xl transition-colors pl-1"
													>
														<div className="w-1/3 flex items-center">
															<Badge className="overflow-hidden text-ellipsis mr-2">
																{event.uid}
															</Badge>
															<Button
																variant="ghost"
																size="icon"
																className="h-4 w-4"
																onClick={() => copyToClipboard(event.uid)}
															>
																<Copy className="h-3 w-3 text-neutral-10" />
															</Button>
														</div>
														<div className="w-1/3 whitespace-nowrap overflow-hidden overflow-ellipsis text-xs text-neutral-10 pr-2">
															{event.source_metadata?.name || 'Rest API'}
														</div>
														<div className="w-1/5">
															{event.is_duplicate_event && (
																<Badge variant="outline" className="text-xs">
																	Duplicate
																</Badge>
															)}
														</div>
														<div className="flex items-center justify-end text-neutral-10 text-xs">
															{new Date(event.created_at).toLocaleTimeString(
																'en-US',
																{ hour: 'numeric', minute: '2-digit' },
															)}
														</div>
													</li>
												))}
											</ul>
										</>
									)
								)}
							</>
						)}

						{/* Event deliveries */}
						{!eventsDetailsItem?.is_duplicate_event && (
							<>
								<p className="text-xs text-neutral-10 font-medium mb-4 mt-4">
									Deliveries Overview
								</p>

								{isLoadingSidebarDeliveries ? (
									<div>
										<Skeleton className="h-4 w-20 rounded-full mb-8" />
										<div className="flex justify-between mb-5">
											<Skeleton className="h-4 w-20 rounded-full" />
											<Skeleton className="h-4 w-52 rounded-full" />
											<Skeleton className="h-4 w-16 rounded-full" />
										</div>
										<div className="flex justify-between mb-5">
											<Skeleton className="h-4 w-20 rounded-full" />
											<Skeleton className="h-4 w-52 rounded-full" />
											<Skeleton className="h-4 w-16 rounded-full" />
										</div>
										<div className="flex justify-between mb-5">
											<Skeleton className="h-4 w-20 rounded-full" />
											<Skeleton className="h-4 w-52 rounded-full" />
											<Skeleton className="h-4 w-16 rounded-full" />
										</div>
									</div>
								) : (
									<>
										{sidebarEventDeliveries.length === 0 ? (
											<div className="border-b border-new.primary-25 mb-6 p-6 pl-0 w-full text-xs text-neutral-10">
												No event delivery attempt for this event yet.
											</div>
										) : (
											<ul className="border-b border-new.primary-25 mb-6">
												{sidebarEventDeliveries.map(delivery => (
													<li
														key={delivery.uid}
														className="cursor-pointer border-none flex justify-between mb-2.5 hover:bg-new.primary-25 py-1.5 rounded-xl transition-colors pl-1"
														onClick={() => {
															// Navigate to event delivery details
															// TO DO: Implement navigation to event delivery details
														}}
													>
														<div className="flex items-center">
															<div className="flex items-center mr-3">
																<Badge
																	className={`shadow-none font-normal text-xs border-0 !rounded-22px py-1.5 px-3 ${setTagColour(delivery.status)}`}
																>
																	{delivery.status}
																</Badge>
																{delivery.device_id && (
																	<svg width="16" height="14" className="mr-1">
																		<use xlinkHref="#cli-icon"></use>
																	</svg>
																)}
															</div>

															<div className="whitespace-nowrap overflow-ellipsis overflow-hidden text-neutral-10 text-center text-xs">
																{!delivery.device_id ? (
																	<div className="flex items-center">
																		{project?.type == 'incoming' && (
																			<div>
																				<div className="max-w-[100px] truncate">
																					{delivery.source_metadata?.name ||
																						'Rest API'}
																				</div>

																				<div className="px-4 font-light">→</div>
																			</div>
																		)}

																		<div className="max-w-[100px] overflow-hidden overflow-ellipsis">
																			{delivery.endpoint_metadata?.name}
																		</div>
																	</div>
																) : (
																	<span>
																		{delivery.device_metadata?.host_name}
																	</span>
																)}
															</div>
														</div>

														<div className="flex items-center justify-end text-neutral-10 text-xs self-end">
															{new Date(delivery.created_at).toLocaleTimeString(
																'en-US',
																{ hour: 'numeric', minute: '2-digit' },
															)}

															<Button variant="ghost" className="pr-0">
																<ArrowRight className="h-6 w-6 fill-neutral-10" />
															</Button>
														</div>
													</li>
												))}
											</ul>
										)}
									</>
								)}
							</>
						)}

						{/* Event payload and headers */}
						{isLoadingEvents ? (
							<>
								<Skeleton className="h-[120px] w-full mb-5" />
								<Skeleton className="h-[120px] w-full" />
							</>
						) : (
							displayedEvents?.length > 0 &&
							eventsDetailsItem && (
								<>
									<div className="mb-4">
										<div className="mb-2 font-medium text-xs">Event</div>
										<SyntaxHighlighter
											language="json"
											style={vs}
											showLineNumbers={true}
											className="rounded-md text-sm"
										>
											{formatCode(
												eventsDetailsItem.data ||
													eventsDetailsItem?.metadata?.data,
											)}
										</SyntaxHighlighter>
									</div>

									{eventsDetailsItem.headers && (
										<div>
											<div className="mb-2 font-medium text-xs">Headers</div>
											<SyntaxHighlighter
												language="json"
												style={vs}
												showLineNumbers={true}
												className="rounded-md text-sm"
											>
												{formatCode(eventsDetailsItem.headers)}
											</SyntaxHighlighter>
										</div>
									)}
								</>
							)
						)}
					</div>
				</div>
			)}

			{/* Pagination */}
			{/* {events?.pagination?.has_next_page ||
			events?.pagination?.has_prev_page ? (
				<div className="flex justify-center items-center gap-2 py-4">
					<Button
						variant="outline"
						size="sm"
						disabled={!events?.pagination?.has_prev_page}
						onClick={() =>
							navigate({
								to: Route.fullPath,
								search: {
									...search,
									direction: 'prev',
									next_page_cursor: events?.pagination?.next_page_cursor || '',
								},
							})
						}
					>
						Previous
					</Button>
					<Button
						variant="outline"
						size="sm"
						disabled={!events?.pagination?.has_next_page}
						onClick={() =>
							navigate({
								to: Route.fullPath,
								search: {
									...search,
									direction: 'next',
									next_page_cursor: events?.pagination?.next_page_cursor || '',
								},
							})
						}
					>
						Next
					</Button>
				</div>
			) : null} */}

			{/* Batch retry dialog */}
			<Dialog
				open={showBatchRetryDialog}
				onOpenChange={setShowBatchRetryDialog}
			>
				<DialogContent className="sm:max-w-md">
					<div className="text-center py-8">
						<img
							src="/assets/img/filter.gif"
							alt="filter icon"
							className="w-[50px] m-auto mb-4"
						/>
						<DialogHeader>
							<DialogTitle className="text-center text-base font-medium text-neutral-11 mb-2">
								The filters applied will affect
							</DialogTitle>
						</DialogHeader>
						<p className="text-center text-base font-semibold mb-8">
							{batchRetryCount || 0} event{batchRetryCount !== 1 ? 's' : ''}
						</p>
						<div className="flex flex-col gap-2">
							<Button
								onClick={batchReplayEvent}
								disabled={isRetrying || batchRetryCount === 0}
								className="m-auto"
							>
								{isRetrying ? 'Retrying Events...' : 'Yes, Apply'}
							</Button>
							<Button
								variant="ghost"
								className="font-semibold m-auto"
								onClick={() => setShowBatchRetryDialog(false)}
							>
								No, Cancel
							</Button>
						</div>
					</div>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
