import { useState, useEffect } from 'react';
import { formatInTimeZone } from 'date-fns-tz';
import { subDays, setHours, setMinutes, format } from 'date-fns';
import { createFileRoute, useNavigate } from '@tanstack/react-router';

import { Copy, Clock7, CalendarIcon } from 'lucide-react';

import {
	Popover,
	PopoverTrigger,
	PopoverContent,
	PopoverClose,
} from '@/components/ui/popover';
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from '@/components/ui/select';
import {
	type ChartConfig,
	ChartContainer,
	ChartTooltip,
	ChartTooltipContent,
} from '@/components/ui/chart';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { DashboardLayout } from '@/components/dashboard';
import { Bar, BarChart, CartesianGrid, XAxis } from 'recharts';

import { cn } from '@/lib/utils';
import { useProjectStore } from '@/store';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as sourcesService from '@/services/sources.service';
import * as projectsService from '@/services/projects.service';
import * as eventsService from '@/services/events.service';

import type { DateRange } from 'react-day-picker';

import appsIcon from '../../../../../assets/svg/apps-icon.svg';
import messageIcon from '../../../../../assets/svg/message-icon.svg';
import emptyStateImg from '../../../../../assets/svg/events-empty-state-image.svg';

const chartData = [
	{ month: 'January', desktop: 186 },
	{ month: 'February', desktop: 0 },
	{ month: 'March', desktop: 237 },
	{ month: 'April', desktop: 73 },
	{ month: 'May', desktop: 209 },
	{ month: 'June', desktop: 214 },
	{ month: 'January', desktop: 0 },
	{ month: 'February', desktop: 305 },
	{ month: 'March', desktop: 237 },
	{ month: 'April', desktop: 0 },
	{ month: 'May', desktop: 209 },
	{ month: 'June', desktop: 214 },
];

const chartConfig = {
	desktop: {
		label: 'Desktop',
		color: '#477db3b3',
	},
} satisfies ChartConfig;

export const Route = createFileRoute('/projects_/$projectId/events/')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	async loader() {
		const stats = await projectsService.getStats();
		const sources = await sourcesService.getSources();
		const latestDeliveries = await eventsService.getEventDeliveries();

		return {
			stats,
			latestSource: sources.content[sources.content.length - 1],
			latestDeliveries,
		};
	},
	component: EventsDeliveriesPage,
});

function EventsDeliveriesPage() {
	const { project } = useProjectStore();
	const navigate = useNavigate();
	const { stats, latestSource, latestDeliveries } = Route.useLoaderData();
	const [date, setDate] = useState<DateRange | undefined>({
		from: subDays(new Date(), 30),
		to: new Date(),
	});
	const [endTimeValue, setEndTimeValue] = useState('23:59');
	const [startTimeValue, setStartTimeValue] = useState('00:00');
	const [dashboardFrequency, setDashboardFrequency] = useState('daily');
	const [dashboardSummary, setDashboardSummary] = useState(null)

	// useEffect(() => {
	// 	let startDate = date?.from ? new Date(search.startDate).toISOString().split('.')[0]
	// 	: undefined
	// 	eventsService.getDashboardSummary(

	// 	)
	
	// 	return () => {
			
	// 	}
	// }, [date])
	

	function setTimeOnDate(time: string, date: Date) {
		const [hours, minutes] = time.split(':').map(str => parseInt(str, 10));
		return setHours(setMinutes(new Date(date), minutes), hours);
	}

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
			{/* Has no events */}
			{!stats.events_exist && (
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center gap-y-6">
						<img
							src={emptyStateImg}
							alt="No subscriptions created"
							className="h-40"
						/>

						<h2 className="font-bold mb-4 text-base text-neutral-sm text-center">
							{project?.type == 'incoming'
								? 'You have no incoming events.'
								: 'You have no outgoing events yet'}
						</h2>

						{stats.subscriptions_exist ? (
							<p className="text-neutral-11 text-xs max-w-[410px] text-center">
								{project?.type == 'incoming'
									? `Start receiving webhooks by adding your webhook URL into your webhook sender platform "${latestSource?.name}"`
									: `You have not sent any webhook events yet. Learn how to do that in our docs`}
							</p>
						) : (
							<p className="text-neutral-11 text-xs max-w-[410px] text-center">
								{project?.type == 'incoming'
									? 'You need to create an endpoint subscribe it to an event source (sender of your webhooks) to start receiving events'
									: 'You need to create an endpoint and subscribe it to listen to events'}
							</p>
						)}

						{stats.subscriptions_exist && project?.type == 'incoming' && (
							<div className="w-full flex flex-col gap-y-6">
								<div className="flex items-center justify-between rounded-8px py-2 border px-4 border-neutral-a3 bg-new.primary-25/50 gap-x-6 w-full">
									<p className="text-xs text-neutral-10 ml-10">
										{latestSource.url}
									</p>
									<Button
										size={'icon'}
										variant={'ghost'}
										className="hover:bg-transparent"
										onClick={() =>
											navigator.clipboard.writeText(latestSource.url || '')
										}
									>
										<Copy className="stroke-neutral-10" />
									</Button>
								</div>

								{latestDeliveries.content.length == 0 && (
									<div className="flex flex-col gap-y-6 w-full items-center">
										<div className="flex items-center justify-center rounded-8px py-4 border px-4 border-neutral-a3 gap-x-4 w-4/5">
											<Clock7 className="w-4 h-4  rounded-[100%] bg-new.primary-25 stroke-new.primary-400" />
											<p className="text-xs text-neutral-10">
												Waiting on your first webhook event
											</p>
										</div>

										<Button
											variant="link"
											className="text-new.primary-400 text-base hover:no-underline"
											asChild
										>
											<a
												href="https://docs.getconvoy.io/guides/receiving-webhook-example"
												target="_blank"
												rel="noreferrer"
												className="text-new.primary-400 text-base"
											>
												Don&apos;t See Your Events Yet?
											</a>
										</Button>
									</div>
								)}

								{latestDeliveries.content.length > 0 && (
									<div className="w-full max-w-[500px]">
										<div className="flex w-full border-b border-b-neutral-a3 text-neutral-11 text-sm p-[10px]">
											<div className="w-1/5 text-left">Status</div>
											<div className="w-1/3 text-left ml-2px">Subscription</div>
											<div className="w-1/5 text-left ml-2px">Event Time</div>
											<div className="w-1/5 text-left ml-2px">Retry Time</div>
										</div>
										{latestDeliveries.content.map(event => (
											<div
												key={event.uid}
												onClick={() => {
													navigate({
														to: '/projects/$projectId/events/event-deliveries/$eventId',
														params: {
															projectId: project.uid,
															eventId: event.uid,
														},
													});
												}}
												className="hover:bg-neutral-a3 cursor-pointer"
											>
												<div className="flex text-left text-sm p-[10px]">
													<div className="w-1/5">
														<Badge
															className={`shadow-none font-normal text-xs border-0 !rounded-22px py-1.5 px-3 ${setTagColour(event.status)}`}
														>
															{event.status}
														</Badge>
													</div>
													<div className="w-1/3">
														{project.type == 'incoming' && (
															<div>
																<span className="max-w-[146px] overflow-hidden overflow-ellipsis">
																	{event?.source_metadata?.name || 'Rest API'}
																</span>
																<span className="px-20px font-light">→</span>
															</div>
														)}

														<span
															className={`${project.type == 'incoming' ? 'max-w-[140px] overflow-hidden overflow-ellipsis' : 'w-[156px] overflow-hidden overflow-ellipsis'}`}
														>
															{event.endpoint_metadata.title ||
																event.endpoint_metadata.name}
														</span>
													</div>
													<div className="w-1/5 ml-2px">
														{Intl.DateTimeFormat('en-GB', {
															timeStyle: 'medium',
															hour12: true,
														}).format(new Date(event.created_at))}
													</div>
													<div className="w-1/5 ml-2px">
														{Intl.DateTimeFormat('en-GB', {
															timeStyle: 'medium',
															hour12: true,
														}).format(new Date(event.updated_at))}
													</div>
												</div>

												<Button
													variant={'ghost'}
													onClick={e => {
														e.stopPropagation();
														console.log('continue to dashboard');
													}}
												>
													Continue to Dashboard
												</Button>
											</div>
										))}
									</div>
								)}
							</div>
						)}

						{!stats.subscriptions_exist && (
							<Button
								variant={'ghost'}
								className="text-xs bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100"
							>
								Complete Project Setup{' '}
								<svg width="24" height="24" className="ml-8px fill-white-100">
									<use xlinkHref="#arrow-right-icon"></use>
								</svg>
							</Button>
						)}

						{stats.subscriptions_exist && project?.type == 'outgoing' && (
							<Button asChild>
								<a
									href="https://docs.getconvoy.io/guides/sending-webhook-example"
									target="_blank"
									rel="noreferrer"
									referrerPolicy="no-referrer"
									className="mt-48px"
								>
									Go to documentation
									<svg className="ml-8px fill-primary-100 w-20px h-20px">
										<use xlinkHref="#external-link-icon"></use>
									</svg>
								</a>
							</Button>
						)}
					</div>
				</div>
			)}

			{/* Has events */}
			{stats.events_exist && (
				<div className="p-6">
					<div className="flex flex-col justify-center gap-y-6">
						<h2 className="text-12 font-medium text-neutral-10 mb-16px">
							Events Summary
						</h2>
						<div className="flex items-center gap-x-4">
							{/* Date range selector */}
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
															onClick={() => setDate(undefined)}
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
							{/* Dashboard Frequency */}
							<div className="flex flex-col gap-y-2">
								<label
									htmlFor="select_filter_period"
									className="text-neutral-9 text-xs"
								>
									Filter by
								</label>
								<Select
									defaultValue={dashboardFrequency}
									onValueChange={setDashboardFrequency}
								>
									<SelectTrigger
										className="w-[100px] h-9 shadow-none focus:ring-0 text-xs text-neutral-9"
										id="select_filter_period"
									>
										<SelectValue
											className="text-xs text-neutral-9"
											placeholder="Sort"
										/>
									</SelectTrigger>
									<SelectContent>
										{['daily', 'weekly', 'monthly', 'yearly'].map(v => (
											<SelectItem
												className="text-xs hover:cursor-pointer"
												value={v}
												key={v}
											>
												<span className="text-neutral-9 capitalize">{v}</span>
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
						</div>
						<div className="border rounded-8px">
							<div className="flex justify-start items-center">
								<div className="flex items-center gap-x-8 pl-4 py-4 border-r p-1">
									<img src={messageIcon} alt="events sent" />
									<div className="flex flex-col justify-center">
										<p>10</p>
										<p className="font-normal text-sm">Events sent</p>
									</div>
									<img
										src={messageIcon}
										alt="events sent"
										className="opacity-50"
									/>
								</div>
								<div className="flex items-center gap-x-8 pl-4 py-4 border-r p-1">
									<img src={appsIcon} alt="events sent" />
									<div className="flex flex-col justify-center">
										<p>8</p>
										<p className="font-normal text-sm">Endpoints</p>
									</div>
									<img src={appsIcon} alt="endpoints" className="opacity-50" />
								</div>
							</div>
							<div className="border-t px-4">
								<ChartContainer
									config={chartConfig}
									className="h-[150px] w-full"
								>
									<BarChart accessibilityLayer data={chartData}>
										<CartesianGrid vertical={false} />
										<XAxis
											dataKey="month"
											tickLine={false}
											tickMargin={10}
											axisLine={true}
											// tickFormatter={value => value.slice(0, 3)}
										/>
										<ChartTooltip
											cursor={false}
											content={<ChartTooltipContent hideLabel />}
										/>
										<Bar
											dataKey="desktop"
											fill="var(--color-desktop)"
											radius={[4, 4, 0, 0]}
										/>
									</BarChart>
								</ChartContainer>
							</div>
						</div>
					</div>
				</div>
			)}
		</DashboardLayout>

		// <DashboardLayout showSidebar={true}>
		// 	<div className="p-6">
		// 		<section className="space-y-6">
		// 			<h1 className="text-lg font-bold text-neutral-12">
		// 				Event Deliveries
		// 			</h1>
		// 		</section>
		// 	</div>
		// </DashboardLayout>
	);
}
