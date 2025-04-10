import { z } from 'zod';
import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { format, setHours, setMinutes } from 'date-fns';
import {formatInTimeZone} from 'date-fns-tz'

import { createFileRoute } from '@tanstack/react-router';

import { ChevronDown, Check, CalendarIcon } from 'lucide-react';

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

import { cn } from '@/lib/utils';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as eventsService from '@/services/events.service';
import * as sourcesService from '@/services/sources.service';

import type { DateRange } from 'react-day-picker';

import searchIcon from '../../../../assets/svg/search-icon.svg';
import eventsLogEmptyStateImg from '../../../../assets/svg/events-empty-state-image.svg';

const EventsLogSearchSchema = z.object({
	sort: z.enum(['asc', 'desc']).catch('desc'),
	next_page_cursor: z.string().catch('FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF'),
	direction: z.enum(['next', 'prev']).optional().catch('next'),
	tail_events: z.boolean().catch(false),
	source_id: z.string().catch(''),
	startDate: z.string().optional().catch(''),
	endDate: z.string().optional().catch(''),
	query: z.string().catch(''),
});

export const Route = createFileRoute('/projects_/$projectId/events-log')({
	component: RouteComponent,
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

const filterFormSchema = z.object({
	sort: z.enum(['asc', 'desc']).optional(),
	tail_events: z.boolean().optional(),
	source_id: z.string().optional(),
	starttDate: z.string().optional(),
	endDate: z.string().optional(),
	query: z.string().optional(),
});

function RouteComponent() {
	const navigate = Route.useNavigate();
	const search = Route.useSearch();
	const { events, sources } = Route.useLoaderData();
	const [loadedSources, setLoadedSources] = useState(sources.content);
	const [isLoadingEvents, setIsLoadingEvents] = useState(false);
	const [endTimeValue, setEndTimeValue] = useState('23:59');
	const [startTimeValue, setStartTimeValue] = useState('00:00');
	const [searchString, setSearchString] = useState(search.query);
	const [date, setDate] = useState<DateRange | undefined>(
		!search.startDate || !search.endDate
			? undefined
			: {
					from: new Date(search.startDate),
					to: new Date(search.endDate),
				},
	);
	useEffect(() => {
		setLoadedSources(sources.content);
	}, [sources]);

	const filterForm = useForm({
		resolver: zodResolver(filterFormSchema),
		defaultValues: {
			sort: 'desc',
			tail_events: false,
			source_id: '',
			startDate: '',
			endDate: '',
			query: '',
		},
	});

	function setTimeOnDate(time: string, date: Date) {
		const [hours, minutes] = time.split(':').map(str => parseInt(str, 10));
		return setHours(setMinutes(new Date(date), minutes), hours);
	}

	return (
		<DashboardLayout showSidebar={true}>
			<div className="p-6">
				<section className="py-6 w-full max-w-[1440px]">
					<h1 className="text-lg font-bold text-neutral-12">Events Log</h1>
				</section>

				{/* Filter Section */}
				<div className="flex items-center justify-between">
					<div className="flex items-center gap-x-4">
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
								defaultValue={'desc'}
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
									<SelectItem className="text-xs " value="asc">
										<span className="text-neutral-9">Asc</span>
									</SelectItem>
									<SelectItem className="text-xs " value="desc">
										<span className="text-neutral-9">Desc</span>
									</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div>
							<div className="px-2 py-1 border border-neutral-5 rounded-8px mt-6">
								<ConvoyCheckbox
									isChecked={false}
									label="Tail Events"
									onChange={e =>
										navigate({
											to: Route.fullPath,
											search: { ...search, tail_events: e.target.checked },
										})
									}
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
											<CalendarIcon />
											{date?.from ? (
												date.to ? (
													<>
														{formatInTimeZone(date.from, "UTC", 'dd/LL/y, h:mm aa')} -{' '}
														{formatInTimeZone(date.to, "UTC", 'dd/LL/y, h:mm aa')}
													</>
												) : (
													format(date.from, 'dd/LL/y, h:mm aa')
												)
											) : (
												<span>Pick a date range</span>
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
															setDate({from, to})
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
							asChild
							variant="ghost"
							className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400 h-[36px]"
						>
							Batch Retry
						</Button>
					</div>
				</div>
			</div>

			{events.content.length == 0 && (
				<div className="m-auto">
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
			)}
		</DashboardLayout>
	);
}
