import { z } from 'zod';
import { useState, useEffect, useCallback, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';
import { Editor, DiffEditor } from '@monaco-editor/react';

import {
	ChevronDown,
	Check,
	EllipsisVertical,
	PencilLine,
	Trash2,
	Copy,
	Info,
	X,
	ChevronRight,
	Save,
} from 'lucide-react';

import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import {
	DropdownMenu,
	DropdownMenuTrigger,
	DropdownMenuContent,
	DropdownMenuItem,
} from '@/components/ui/dropdown-menu';
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from '@/components/ui/tooltip';
import {
	Dialog,
	DialogContent,
	DialogTitle,
	DialogDescription,
	DialogClose,
	DialogHeader,
	DialogFooter,
} from '@/components/ui/dialog';
import {
	Popover,
	PopoverTrigger,
	PopoverContent,
	PopoverClose,
} from '@/components/ui/popover';
import {
	Sheet,
	SheetClose,
	SheetContent,
	SheetDescription,
	SheetFooter,
	SheetHeader,
	SheetTitle,
} from '@/components/ui/sheet';
import {
	Command,
	CommandInput,
	CommandList,
	CommandEmpty,
	CommandGroup,
	CommandItem,
} from '@/components/ui/command';
import { Command as CommandPrimitive } from 'cmdk';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ConvoyCheckbox } from '@/components/convoy-checkbox';
import {
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
	Table,
} from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { DashboardLayout } from '@/components/dashboard';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

import { cn } from '@/lib/utils';
import { useLicenseStore, useProjectStore } from '@/store';
import {
	groupItemsByDate,
	stringToJson,
	transformSourceValueType,
} from '@/lib/pipes';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as sourcesService from '@/services/sources.service';
import { getUserPermissions } from '@/services/auth.service';
import * as projectsService from '@/services/projects.service';
import * as endpointsService from '@/services/endpoints.service';
import * as subscriptionsService from '@/services/subscriptions.service';

import type { KeyboardEvent } from 'react';
import type { SUBSCRIPTION } from '@/models/subscription.model';

import searchIcon from '../../../../../assets/svg/search-icon.svg';
import warningAnimation from '../../../../../assets/img/warning-animation.gif';
import subscriptionsEmptyStateImg from '../../../../../assets/img/subscriptions-empty-state.png';

const SubscriptionsSearchSchema = z.object({
	endpointId: z.string().catch(''),
	next_page_cursor: z.string().catch('FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF'),
	direction: z.enum(['next', 'prev']).optional().catch('next'),
	name: z.string().optional().catch(''),
	subscription_name: z.string().catch(''),
});

const EndpointsSearchSchema = z.object({
	endpoint_id: z.string(),
});

// TODO form vaidation
const endpointSchema = z.object({
	endpoint_id: z.string().optional(),
	name: z.string().min(1, { message: 'Please provide a name' }),
	url: z.string().url({ message: 'Invalid endpoint URL' }),
	support_email: z
		.string()
		.email({ message: 'Email is invalid' })
		.optional()
		.or(z.literal('')),
	slack_webhook_url: z
		.string()
		.url({ message: 'URL is invalid' })
		.optional()
		.or(z.literal('')),
	secret: z.string().optional(),
	http_timeout: z.number().optional(),
	description: z.string().optional(),
	owner_id: z.string().optional(),
	rate_limit: z.number().optional(),
	rate_limit_duration: z.number().optional(),
	authentication: z
		.object({
			type: z.string(),
			api_key: z.object({
				header_name: z.string(),
				header_value: z.string(),
			}),
		})
		.optional(),
	advanced_signatures: z.boolean().nullable(),
	function: z
		.string()
		.nullable()
		.transform(v => (v?.length ? v : null)), // transform function
	filter_config: z.object({
		event_types: z
			.array(z.string())
			.nullable()
			.transform(v => (v?.length ? v : null))
			.transform(v => (v?.length ? v : null)),
		filter: z.object({
			headers: z.any().transform(v => {
				const isEmptyObj =
					typeof v === 'object' && v !== null && Object.keys(v).length == 0;
				if (isEmptyObj) return null;
				return v;
			}),
			body: z.any().transform(v => {
				const isEmptyObj =
					typeof v === 'object' && v !== null && Object.keys(v).length == 0;
				if (isEmptyObj) return null;
				return v;
			}),
		}),
	}),
	useExistingEndpoint: z.boolean(),
	showHttpTimeout: z.boolean(),
	showRateLimit: z.boolean(),
	showOwnerId: z.boolean(),
	showAuth: z.boolean(),
	showNotifications: z.boolean(),
	showEventsFilter: z.boolean(),
	showEventTypes: z.boolean(),
	showTransform: z.boolean(),
	showSignatureFormat: z.boolean(),
});

export const Route = createFileRoute('/projects_/$projectId/subscriptions/')({
	component: ListSubcriptionsPage,
	validateSearch: SubscriptionsSearchSchema,
	beforeLoad({ context, search }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
		return { search };
	},
	loader: async ({ context, params }) => {
		const perms = await getUserPermissions();
		const { search } = context;
		const subscriptions = await subscriptionsService.getSubscriptions(search);
		const endpoints = await endpointsService.getEndpoints();
		const { event_types } = await projectsService.getEventTypes(
			params.projectId,
		);

		return {
			canManageSubscriptions: perms.includes('Subscriptions|MANAGE'),
			subscriptions,
			endpoints: endpoints.data.content,
			eventTypes: event_types
				.filter(et => et.deprecated_at === null)
				.map(({ name }) => name),
		};
	},
});

const defaultOutput = { previous: '', current: '' };

const defaultFilterRequestBody = `{
		"id": "Sample-1",
		"name": "Sample 1",
		"description": "This is sample data #1"
}`;

const defaultBody = {
	id: 'Sample-1',
	name: 'Sample 1',
	description: 'This is sample data #1',
};

type FuncOutput = {
	previous: Record<string, unknown> | null | string;
	current: Record<string, unknown> | null | string;
};

const editorOptions = {
	formatOnType: true,
	formatOnPaste: true,
	minimap: { enabled: false },
	scrollBeyondLastLine: false,
	fontSize: 12,
};

function ListSubcriptionsPage() {
	const { canManageSubscriptions, subscriptions, endpoints, eventTypes } =
		Route.useLoaderData();
	const search = Route.useSearch();
	const { project } = useProjectStore();
	const navigate = useNavigate({ from: Route.fullPath });
	const { projectId } = Route.useParams();
	const { licenses } = useLicenseStore();
	const [loadedEndpoints, setLoadedEndpoints] = useState(endpoints);
	const [loadedSubscriptions, setLoadedSubscriptions] = useState(subscriptions);
	const [searchString, setSearchString] = useState(search.name);
	const [isDeletingSub, setIsDeletingSub] = useState(false);
	const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
	const [isViewDetailsDialogOpen, setIsViewDetailsDialogOpen] = useState(false);
	const [currentSub, setCurrentSub] = useState<null | SUBSCRIPTION>(null);
	const [isMultiSelectOpen, setIsMultiSelectOpen] = useState(false);
	const [selectedEventTypes, setSelectedEventTypes] = useState<string[]>([]);

	const [inputValue, setInputValue] = useState('');

	const handleUnselect = useCallback((eventType: string) => {
		setSelectedEventTypes(prev => prev.filter(s => s !== eventType));
	}, []);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLInputElement>) => {
			if (e.key === 'Backspace' && selectedEventTypes.length > 0) {
				setSelectedEventTypes(prev => prev.slice(0, -1));
				return true;
			}
			return false;
		},
		[selectedEventTypes],
	);

	const filteredEventTypes = useMemo(
		() => eventTypes.filter(et => !selectedEventTypes.includes(et)),
		[selectedEventTypes, eventTypes],
	);

	const [showEventsFilterDialog, setShowEventsFilterDialog] = useState(false);
	const [hasPassedTestFilter, setHasPassedTestFilter] = useState(false);
	const [eventsFilter, setEventsFilter] = useState({
		request: {
			body: defaultFilterRequestBody,
			header: '',
		},
		schema: {
			body: `null`,
			header: `null`,
		},
	});

	const [showTransformFunctionDialog, setShowTransformFunctionDialog] =
		useState(false);
	// Transform function state variables
	// TODO use a reducer hook
	const [isTestingFunction, setIsTestingFunction] = useState(false);
	const [isTransformPassed, setIsTransformPassed] = useState(false);
	const [showConsole, setShowConsole] = useState(true);
	const [transformBodyPayload, setTransformBodyPayload] =
		useState<Record<string, unknown>>(defaultBody);
	const [headerPayload, setHeaderPayload] =
		useState<Record<string, unknown>>(defaultBody);
	const [transformFnBody, setTransformFnBody] = useState<string>(
		`/*  1. While you can write multiple functions, the main
		 function called for your transformation is the transform function.
	
	2. The only argument acceptable in the transform function is the
	 payload data.
	
	3. The transform method must return a value.
	
	4. Console logs lust be written like this:
	console.log('%j', logged_item) to get printed in the log below.
	
	5. The output payload from the function should be in this format
			{
					"owner_id": "string, optional",
					"event_type": "string, required",
					"data": "object, required",
					"custom_headers": "object, optional",
					"idempotency_key": "string, optional"
					"endpoint_id": "string, depends",
			}
	
	6. The endpoint_id field is only required when sending event to
	a single endpoint. */
	
	function transform(payload) {
			// Transform function here
			return {
					"endpoint_id": "",
					"owner_id": "",
					"event_type": "sample",
					"data": payload,
					"custom_headers": {
							"sample-header": "sample-value"
					},
					"idempotency_key": ""
			}
	}`,
	);
	const [transformFnHeader, setTransformFnHeader] = useState<string>(
		`/* 1. While you can write multiple functions, the main function
	called for your transformation is the transform function.
	
	2. The only argument acceptable in the transform function is
	the payload data.
	
	3. The transform method must return a value.
	
	4. Console logs lust be written like this
	console.log('%j', logged_item) to get printed in the log below. */
	
	function transform(payload) {
	// Transform function here
	return payload;
	}`,
	);
	const [bodyOutput, setBodyOutput] = useState<FuncOutput>(defaultOutput);
	const [headerOutput, setHeaderOutput] = useState<FuncOutput>(defaultOutput);
	const [bodyLogs, setBodyLogs] = useState<string[]>([]);
	const [headerLogs, setHeaderLogs] = useState<string[]>([]);
	const [transformFn, setTransformFn] = useState<string>('');
	// const [headerTransformFn, setHeaderTransformFn] = useState<string>();
	const [hasSavedFn, setHasSavedFn] = useState(false);
	useEffect(() => {
		setLoadedSubscriptions(subscriptions);
	}, [subscriptions]);

	useEffect(() => {
		setLoadedEndpoints(endpoints);
	}, [endpoints]);

	const endpointSearchForm = useForm<z.infer<typeof EndpointsSearchSchema>>({
		resolver: zodResolver(EndpointsSearchSchema),
		defaultValues: { endpoint_id: '' },
	});

	const endpointForm = useForm<z.infer<typeof endpointSchema>>({
		resolver: zodResolver(endpointSchema),
		defaultValues: {
			endpoint_id: '',
			name: '',
			url: '',
			secret: '',
			owner_id: '',
			// @ts-expect-error the transform takes care of this
			http_timeout: '',
			// @ts-expect-error the transform takes care of this
			rate_limit: '',
			// @ts-expect-error the transform takes care of this
			rate_limit_duration: '',
			support_email: '',
			slack_webhook_url: '',
			authentication: {
				type: 'api_key',
				api_key: {
					header_name: '',
					header_value: '',
				},
			},
			// @ts-expect-error thr transformation takes care of this
			advanced_signatures: 'true',
			showHttpTimeout: false,
			showRateLimit: false,
			showOwnerId: false,
			showAuth: false,
			showNotifications: false,
			showSignatureFormat: false,
			useExistingEndpoint: true,
			showTransform: false,
			showEventTypes: false,
			showEventsFilter: licenses.includes('ADVANCED_SUBSCRIPTIONS'),
		},
	});

	async function handleSearch(e: React.FormEvent) {
		e.preventDefault();
		navigate({
			to: Route.fullPath,
			search: {
				...search,
				name: searchString,
			},
		});
	}

	async function handleSelectEndpoint(endpointId: string) {
		navigate({
			to: Route.fullPath,
			search: {
				...search,
				endpointId,
			},
		});
	}

	async function deleteSubscription(subId: string) {
		setIsDeletingSub(false);
		try {
			await subscriptionsService.deleteSubscription(subId);
		} catch (error) {
			console.error(error);
		} finally {
			setIsDeletingSub(false);
		}
	}

	function hasFilter(filterObject: {
		headers: Record<string, unknown>;
		body: Record<string, unknown>;
	}): boolean {
		return (
			Object.keys(filterObject.body).length > 0 ||
			Object.keys(filterObject.headers).length > 0
		);
	}

	async function testFilter() {
		setHasPassedTestFilter(false);
		try {
			const eventFilter = {
				request: {
					body: stringToJson(eventsFilter.request.body),
					header: stringToJson(eventsFilter.request.header),
				},
				schema: {
					body: stringToJson(eventsFilter.schema.body),
					header: stringToJson(eventsFilter.schema.header),
				},
			};
			const isPass = await subscriptionsService.testFilter(eventFilter);
			setHasPassedTestFilter(isPass);
			endpointForm.setValue(
				'filter_config.filter.body',
				eventFilter.schema.body,
			);
			endpointForm.setValue(
				'filter_config.filter.headers',
				eventFilter.schema.header,
			);
			return eventFilter;
			// TODO show notification
		} catch (error) {
			console.error(error);
			return null;
		}
	}

	// Function to test transform
	async function testTransformFunction(type: 'body' | 'header') {
		setIsTransformPassed(false);
		setIsTestingFunction(true);

		const payload = type === 'body' ? transformBodyPayload : headerPayload;
		const transformFunc = type === 'body' ? transformFnBody : transformFnHeader;

		try {
			// Call the sources service to test the transform function
			const response = await sourcesService.testTransformFunction({
				payload,
				function: transformFunc,
				type,
			});

			// In a real implementation, this would return payload and logs
			if (type === 'body') {
				setBodyOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));

				setBodyLogs(
					response.log.toReversed().length
						? response.log.toReversed()
						: ['Transform function executed successfully'],
				);
			} else {
				setHeaderOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));
				setHeaderLogs(
					response.log.toReversed().length
						? response.log.toReversed()
						: ['Transform function executed successfully'],
				);
			}

			setIsTransformPassed(true);
			setIsTestingFunction(false);
			setShowConsole(bodyLogs.length || headerLogs.length ? true : false);

			if (type === 'body') {
				setTransformFn(transformFunc);
			}

			// else {
			// 	setHeaderTransformFn(transformFunc);
			// 	console.log({ transformFunc });
			// }
		} catch (error) {
			console.error(error);
			setIsTestingFunction(false);
			if (type === 'body') {
				setBodyLogs(['Error executing transform function']);
			} else {
				setHeaderLogs(['Error executing transform function']);
			}
		}
	}

	function saveTransformFn() {
		endpointForm.setValue('function', transformFn);
		endpointForm.trigger('function');
	}

	async function setSubscriptionFilter() {
		const eventFilter = await testFilter();
		if (hasPassedTestFilter && eventFilter) {
			const { schema } = eventFilter;
			endpointForm.setValue('filter_config.filter.body', schema.body);
			endpointForm.setValue('filter_config.filter.headers', schema.header);
		}
	}

	async function handleCreateEndpointForm(val: z.infer<typeof endpointSchema>) {
		console.log(val);
	}

	if (loadedSubscriptions.content.length === 0 && !search.name) {
		return (
			<DashboardLayout showSidebar={true}>
				<div className="m-auto">
					<div className="flex flex-col items-center justify-center">
						<img
							src={subscriptionsEmptyStateImg}
							alt="No subscriptions created"
							className="h-40 mb-6"
						/>
						<h2 className="font-bold mb-4 text-base text-neutral-12 text-center">
							You currently do not have any subscriptions
						</h2>

						<p className="text-neutral-10 text-sm mb-6 max-w-[410px] text-center">
							Webhook subscriptions lets you define the source of your webhook
							and the destination where any webhook event should be sent. It is
							what allows Convoy to identify and proxy your webhooks.
						</p>

						<Button
							className="mt-9 mb-9 hover:bg-new.primary-400 bg-new.primary-400 text-white-100 hover:text-white-100 px-5 py-3 text-xs"
							disabled={!canManageSubscriptions}
							asChild
						>
							<Link
								to="/projects/$projectId/subscriptions/new"
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
								Create a subscription
							</Link>
						</Button>
					</div>
				</div>
			</DashboardLayout>
		);
	}

	return (
		<DashboardLayout showSidebar={true}>
			<div className="p-6">
				<section className="space-y-6">
					<h1 className="text-lg font-bold text-neutral-12">Subscriptions</h1>

					{/* Filter Section */}
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-x-4">
							<div>
								<form className="flex flex-col gap-y-2" onSubmit={handleSearch}>
									<label
										className="text-neutral-9 text-xs"
										htmlFor="search_subscription"
									>
										Search subscription
									</label>
									<div className="border border-primary-400 h-9 px-[14px] py-0 max-w-60 w-full rounded-[10px] flex items-center bg-white-100">
										<img src={searchIcon} alt="search icon" className="mr-2" />
										<input
											type="search"
											id="search_subscription"
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
							<div>
								<Form {...endpointSearchForm}>
									<form>
										<FormField
											control={endpointSearchForm.control}
											name="endpoint_id"
											render={({ field }) => (
												<FormItem className="flex flex-col gap-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Select from existing endpoints
													</FormLabel>
													<Popover>
														<PopoverTrigger
															asChild
															className="shadow-none w-60"
														>
															<FormControl>
																<Button
																	variant="outline"
																	role="combobox"
																	className="flex items-center text-xs text-neutral-10 hover:text-neutral-10"
																>
																	{field.value
																		? [{ name: 'All', uid: '' }]
																				// @ts-expect-error a hack
																				.concat(loadedEndpoints)
																				.find(ep => ep.uid === field.value)
																				?.name
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
																	placeholder="Search endpoints"
																	className="h-9 placeholder:text-xs text-xs"
																/>
																<CommandList className="max-h-40">
																	<CommandEmpty className="text-xs text-neutral-10 hover:text-neutral-10 py-4">
																		No endpoints found.
																	</CommandEmpty>
																	<CommandGroup>
																		{[{ name: 'All', uid: '' }]
																			// @ts-expect-error a hack
																			.concat(loadedEndpoints)
																			.map(ep => (
																				<PopoverClose
																					key={ep.uid}
																					className="flex flex-col w-full"
																				>
																					<CommandItem
																						className="cursor-pointer text-xs !text-neutral-10 py-2 !hover:text-neutral-10"
																						value={`${ep.name}-${ep.uid}`}
																						onSelect={() => {
																							field.onChange(ep.uid);
																							handleSelectEndpoint(ep.uid);
																						}}
																					>
																						{ep.name}
																						<Check
																							className={cn(
																								'ml-auto',
																								ep.uid === field.value
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
								disabled={!canManageSubscriptions}
								variant="ghost"
								className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400 h-[36px]"
							>
								<Link
									to="/projects/$projectId/subscriptions/new"
									params={{ projectId }}
								>
									<svg width="22" height="22" fill="#ffffff">
										<use xlinkHref="#plus-icon"></use>
									</svg>
									Subscription
								</Link>
							</Button>
						</div>
					</div>

					{/* Table */}
					<div>
						<div className="convoy-card bg-white rounded-lg border">
							<div className="overflow-y-auto overflow-x-auto w-full">
								{/* min-h-[70vh] */}
								{/* TODO: make content of table scrollable without scrolling the page and the header*/}
								<Table>
									<TableHeader className="border-b border-b-new.primary-25">
										<TableRow>
											<TableHead className="pl-[20px] uppercase text-xs text-new.black">
												Name
											</TableHead>
											<TableHead className="uppercase text-xs text-new.black w-[100px]">
												Configs
											</TableHead>
											<TableHead></TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{Array.from(
											groupItemsByDate(loadedSubscriptions.content),
										).map(([dateKey, subs]) => {
											return [
												<TableRow
													key={dateKey}
													className="hover:bg-transparent border-new.primary-25 border-t border-b-0 py-2"
												>
													<TableCell className="font-normal text-neutral-10 text-xs bg-neutral-a3">
														{dateKey}
													</TableCell>
													<TableCell className="bg-neutral-a3"></TableCell>
													<TableCell className="bg-neutral-a3"></TableCell>
													<TableCell className="bg-neutral-a3"></TableCell>
												</TableRow>,
											].concat(
												subs.map(sub => (
													<TableRow
														key={sub.uid}
														className="border-b border-b-new.primary-25 duration-300 hover:bg-new.primary-25 transition-all"
													>
														<TableCell className="w-[300px]">
															{project?.type == 'outgoing' && (
																<div className="truncate max-w-[290px] pl-4 text-neutral-12 text-xs py-2">
																	{sub.name}
																</div>
															)}

															{project?.type == 'incoming' && (
																<div className="flex items-center gap-x-2 min-w-[370px] pl-4">
																	<p className="text-xs">
																		{sub.source_metadata.name}
																	</p>

																	<Badge
																		variant={null}
																		className="font-normal text-xs border-0 !rounded-22px py-1 px-3 bg-neutral-a3 ml-2"
																	>
																		{sub.source_metadata.provider ||
																			transformSourceValueType(
																				sub.source_metadata.verifier.type,
																				'verifier',
																			)}
																	</Badge>

																	<span className="px-16px font-light">→</span>

																	<span className="max-w-[150px] truncate text-xs">
																		{sub.endpoint_metadata?.name}
																	</span>
																</div>
															)}
														</TableCell>
														<TableCell className="text-xs w-72">
															<div className="flex items-center gap-x-2 whitespace-nowrap">
																<Badge
																	variant={null}
																	className={cn(
																		'font-normal text-xs border-0 !rounded-22px py-1 px-3',
																		hasFilter(sub.filter_config.filter)
																			? 'bg-new.primary-25'
																			: 'bg-neutral-a3',
																	)}
																>
																	Filter
																</Badge>

																{project?.type == 'outgoing' && (
																	<Badge
																		variant={null}
																		className={cn(
																			'font-normal text-xs border-0 !rounded-22px py-1 px-3',
																			sub.filter_config.event_types?.length >
																				1 ||
																				(sub.filter_config.event_types.length ==
																					1 &&
																					sub.filter_config.event_types[0] !==
																						'*')
																				? 'bg-new.primary-25 text-new.primary-500'
																				: 'bg-neutral-a3',
																		)}
																	>
																		Event Types
																	</Badge>
																)}

																{project?.type == 'incoming' && (
																	<Badge
																		variant={null}
																		className={cn(
																			'font-normal text-xs border-0 !rounded-22px py-1 px-3',
																			sub.function
																				? 'bg-new.primary-25 text-new.primary-400'
																				: 'bg-neutral-a3',
																		)}
																	>
																		Transform
																	</Badge>
																)}
															</div>
														</TableCell>
														<TableCell></TableCell>
														<TableCell className="flex justify-end">
															<DropdownMenu>
																<DropdownMenuTrigger
																	asChild
																	onSelect={() => console.log(sub.uid)}
																>
																	<Button
																		variant="ghost"
																		size="icon"
																		className="hover:bg-transparent focus-visible:ring-0 ml-auto"
																	>
																		<EllipsisVertical className="fill-neutral-10" />
																	</Button>
																</DropdownMenuTrigger>
																<DropdownMenuContent side="left">
																	<DropdownMenuItem
																		className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer"
																		onClick={() => {
																			setCurrentSub(sub);
																			setIsViewDetailsDialogOpen(true);
																		}}
																	>
																		<svg
																			width="24"
																			height="24"
																			className="fill-neutral-10 scale-80"
																		>
																			<use xlinkHref="#shield-icon"></use>
																		</svg>
																		<span className="text-xs text-neutral-9">
																			View Details
																		</span>
																	</DropdownMenuItem>

																	<DropdownMenuItem
																		className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer"
																		disabled={!canManageSubscriptions}
																		onClick={() =>
																			navigate({
																				to: '/projects/$projectId/subscriptions/$subscriptionId',
																				params: {
																					projectId,
																					subscriptionId: sub.uid,
																				},
																			})
																		}
																	>
																		<PencilLine className="stroke-neutral-9 !w-3 !h-3" />
																		<span className="text-xs text-neutral-9">
																			Edit
																		</span>
																	</DropdownMenuItem>

																	<DropdownMenuItem
																		className="flex items-center gap-2 hover:bg-new.primary-50 cursor-pointer"
																		disabled={!canManageSubscriptions}
																		onClick={() => {
																			setCurrentSub(sub);
																			setIsDeleteDialogOpen(true);
																		}}
																	>
																		<Trash2 className="stroke-destructive !w-3 !h-3" />
																		<span className="text-xs text-destructive">
																			Delete
																		</span>
																	</DropdownMenuItem>
																</DropdownMenuContent>
															</DropdownMenu>
														</TableCell>
													</TableRow>
												)),
											);
										})}
									</TableBody>
								</Table>
							</div>

							{/* Pagination */}
							{/* TODO: Add pagination */}
						</div>
					</div>
				</section>
			</div>

			{/* View Subscription Details Sheet */}
			<Sheet
				open={isViewDetailsDialogOpen}
				onOpenChange={() =>
					setIsViewDetailsDialogOpen(!isViewDetailsDialogOpen)
				}
			>
				<SheetContent className="min-w-[480px] p-0 overflow-y-scroll pb-6">
					<SheetHeader className="text-start p-4 pb-0">
						<SheetTitle className="font-semibold text-sm capitalize max-w-[320px] overflow-ellipsis overflow-hidden whitespace-nowrap">
							{currentSub?.name}
						</SheetTitle>
						<SheetDescription className="sr-only">
							{currentSub?.name}
						</SheetDescription>
					</SheetHeader>
					<hr className="my-4 border-neutral-5" />
					<div className="flex flex-col gap-6">
						{/* Source */}
						{project?.type == 'incoming' && (
							<div className="flex flex-col gap-y-4 px-4">
								<p className="text-neutral-10 text-sm">Source</p>
								<div className="border border-neutral-5 flex flex-col justify-center rounded-8px hover:shadow-md">
									<div className="flex flex-col p-4 gap-y-1">
										<p className="text-neutral-11 text-[10px]">
											{currentSub?.source_metadata.provider ||
												transformSourceValueType(
													currentSub?.source_metadata?.verifier?.type as string,
													'verifier',
												)}
										</p>
										<Link
											to="/projects/$projectId/sources"
											params={{ projectId }}
											search={{ id: currentSub?.source_metadata.uid }}
											activeProps={{}}
											className="text-sm flex justify-between items-center"
										>
											<span>github Source</span>
											<ChevronRight size={14} />
										</Link>
									</div>
									<hr />
									<Button
										variant={'ghost'}
										className="p-4 flex items-center justify-start gap-x-2 hover:bg-transparent my-2"
										onClick={() =>
											navigator.clipboard
												.writeText(currentSub?.source_metadata.url as string)
												.then()
										}
									>
										<Copy className="stroke-new.primary-400" size={14} />
										<p className="text-sm font-medium">Copy URL</p>
									</Button>
								</div>
							</div>
						)}
						{/* Endpoint */}
						<div className="flex flex-col gap-y-4 px-4">
							<p className="text-neutral-10 text-sm">Endpoint</p>
							<div className="border border-neutral-5 flex flex-col justify-center rounded-8px hover:shadow-md">
								<div className="flex flex-col p-4 gap-y-1">
									<Link
										to="/projects/$projectId/endpoints/$endpointId"
										params={{
											projectId,
											endpointId: currentSub?.endpoint_metadata?.uid as string,
										}}
										activeProps={{}}
										className="text-sm flex justify-between items-center"
									>
										<span>{currentSub?.endpoint_metadata?.name}</span>
										<ChevronRight size={14} />
									</Link>
								</div>
								<hr />
								<Button
									variant={'ghost'}
									className="p-4 flex items-center justify-start gap-x-2 hover:bg-transparent my-2"
									onClick={() =>
										navigator.clipboard
											.writeText(currentSub?.endpoint_metadata?.url as string)
											.then()
									}
								>
									<Copy className="stroke-new.primary-400" size={14} />
									<p className="text-sm font-medium">Copy URL</p>
								</Button>
							</div>
						</div>
						{/* Endpoint Form */}
						<section className="p-6">
							<h2 className="font-semibold text-sm">Endpoint</h2>
							<p className="text-xs text-neutral-10 mt-1.5">
								Endpoint this subscription routes events into.
							</p>
							<div className="mt-6">
								<Form {...endpointForm}>
									<form
										onSubmit={endpointForm.handleSubmit(
											handleCreateEndpointForm,
										)}
									>
										{endpointForm.watch('useExistingEndpoint') ? (
											<div className="space-y-4">
												<FormField
													control={endpointForm.control}
													name="endpoint_id"
													render={({ field }) => (
														<FormItem className="flex flex-col gap-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																Select from existing endpoints
															</FormLabel>
															<Popover>
																<PopoverTrigger
																	asChild
																	className="shadow-none h-12"
																>
																	<FormControl>
																		<Button
																			variant="outline"
																			role="combobox"
																			className="flex items-center text-xs text-neutral-10 hover:text-neutral-10"
																		>
																			{field.value
																				? endpoints.find(
																						ep => ep.uid === field.value,
																					)?.name
																				: ''}
																			<ChevronDown className="ml-auto opacity-50" />
																		</Button>
																	</FormControl>
																</PopoverTrigger>
																<PopoverContent
																	align="start"
																	className="p-0 shadow-none w-full"
																>
																	<Command className="shadow-none">
																		<CommandInput
																			placeholder="Filter endpoints"
																			className="h-9"
																		/>
																		<CommandList className="max-h-40">
																			<CommandEmpty className="text-xs text-neutral-10 hover:text-neutral-10 py-4">
																				No endpoints found.
																			</CommandEmpty>
																			<CommandGroup>
																				{endpoints.map(ep => (
																					<PopoverClose
																						key={ep.uid}
																						className="flex flex-col w-full"
																					>
																						<CommandItem
																							className="cursor-pointer text-xs !text-neutral-10 py-4 !hover:text-neutral-10"
																							value={`${ep.name}-${ep.uid}`}
																							onSelect={() =>
																								field.onChange(ep.uid)
																							}
																						>
																							{ep.name} ({ep.uid})
																							<Check
																								className={cn(
																									'ml-auto',
																									ep.uid === field.value
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
												<div>
													<FormField
														name="useExistingEndpoint"
														control={endpointForm.control}
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<Button
																		disabled={!canManageSubscriptions}
																		variant="ghost"
																		type="button"
																		size="sm"
																		className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																		onClick={() => field.onChange(!field.value)}
																	>
																		Create New Endpoint
																	</Button>
																</FormControl>
															</FormItem>
														)}
													/>
												</div>
											</div>
										) : (
											<div className="space-y-4">
												<div className="grid grid-cols-2 gap-x-5 gap-y-4">
													<FormField
														name="name"
														control={endpointForm.control}
														render={({ field, fieldState }) => (
															<FormItem className="space-y-2">
																<FormLabel className="text-neutral-9 text-xs">
																	Name
																</FormLabel>
																<FormControl>
																	<Input
																		type="text"
																		autoComplete="text"
																		{...field}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: ' hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																	></Input>
																</FormControl>
																<FormMessageWithErrorIcon />
															</FormItem>
														)}
													/>

													<FormField
														name="url"
														control={endpointForm.control}
														render={({ field, fieldState }) => (
															<FormItem className="space-y-2">
																<FormLabel className="text-neutral-9 text-xs">
																	URL
																</FormLabel>
																<FormControl>
																	<Input
																		type="url"
																		autoComplete="text"
																		{...field}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: ' hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																	></Input>
																</FormControl>
																<FormMessageWithErrorIcon />
															</FormItem>
														)}
													/>

													<FormField
														name="secret"
														control={endpointForm.control}
														render={({ field, fieldState }) => (
															<FormItem className="space-y-2 col-span-full">
																<FormLabel className="text-neutral-9 text-xs">
																	Secret
																</FormLabel>
																<FormControl>
																	<Input
																		type="text"
																		{...field}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: ' hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																	/>
																</FormControl>
																<FormMessageWithErrorIcon />
															</FormItem>
														)}
													/>
												</div>

												<div className="flex items-center gap-x-3 gap-y-4 flex-wrap">
													{/* TODO add popover to show if business and disabled */}
													<FormField
														control={endpointForm.control}
														name="showHttpTimeout"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Timeout"
																		isChecked={field.value}
																		onChange={field.onChange}
																		disabled={
																			!licenses.includes(
																				'ADVANCED_ENDPOINT_MANAGEMENT',
																			)
																		}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													<FormField
														control={endpointForm.control}
														name="showOwnerId"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Owner ID"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													<FormField
														control={endpointForm.control}
														name="showRateLimit"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Rate Limit"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													<FormField
														control={endpointForm.control}
														name="showAuth"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Auth"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													<FormField
														control={endpointForm.control}
														name="showNotifications"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Notifications"
																		isChecked={field.value}
																		onChange={field.onChange}
																		disabled={
																			!licenses.includes(
																				'ADVANCED_ENDPOINT_MANAGEMENT',
																			)
																		}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													{project?.type == 'outgoing' && (
														<FormField
															control={endpointForm.control}
															name="showSignatureFormat"
															render={({ field }) => (
																<FormItem>
																	<FormControl>
																		<ConvoyCheckbox
																			label="Signature Format"
																			isChecked={field.value}
																			onChange={field.onChange}
																		/>
																	</FormControl>
																</FormItem>
															)}
														/>
													)}
												</div>

												{/* HTTP Timeout Section */}
												<div>
													{endpointForm.watch('showHttpTimeout') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<FormField
																control={endpointForm.control}
																name="http_timeout"
																render={({ field, fieldState }) => (
																	<FormItem className="w-full relative mb-2 block">
																		<div className="w-full mb-2 flex items-center justify-between">
																			<FormLabel
																				className="text-xs/5 text-neutral-9"
																				htmlFor="endpoint_http_timeout"
																			>
																				Timeout Value
																			</FormLabel>
																		</div>
																		<FormControl className="w-full relative">
																			<div className="relative">
																				<Input
																					id="endpoint_http_timeout"
																					inputMode="numeric"
																					pattern="\d*"
																					type="number"
																					min={0}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: 'hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																					placeholder="e.g 60"
																					{...field}
																				/>
																				<span className="absolute right-[1%] top-4 h-full px-3 text-xs text-neutral-9">
																					sec
																				</span>
																			</div>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													)}
												</div>
												{/* Owner ID Section */}
												<div>
													{endpointForm.watch('showOwnerId') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<FormField
																name="owner_id"
																control={endpointForm.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			Owner ID
																		</FormLabel>
																		<FormControl>
																			<Input
																				type="text"
																				autoComplete="text"
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			></Input>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													)}
												</div>
												{/* Rate Limit Section */}
												<div>
													{endpointForm.watch('showRateLimit') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<p className="text-xs text-neutral-11 font-medium mb-3">
																Rate Limit
															</p>
															<div className="grid grid-cols-2 gap-x-5">
																<FormField
																	control={endpointForm.control}
																	name="rate_limit_duration"
																	render={({ field, fieldState }) => (
																		<FormItem className="w-full relative space-y-2">
																			<FormLabel
																				className="text-xs/5 text-neutral-9"
																				htmlFor="endpoint_ratelimit_duration"
																			>
																				Duration
																			</FormLabel>
																			<FormControl className="w-full relative">
																				<div className="relative">
																					<Input
																						id="endpoint_ratelimit_duration"
																						inputMode="numeric"
																						pattern="\d*"
																						type="number"
																						min={0}
																						className={cn(
																							'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																							fieldState.error
																								? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																								: 'hover:border-new.primary-100 focus:border-new.primary-300',
																						)}
																						placeholder="e.g 50"
																						{...field}
																					/>
																					<span className="absolute right-[1%] top-4 h-full px-3 text-xs text-neutral-9">
																						sec
																					</span>
																				</div>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>

																<FormField
																	name="rate_limit"
																	control={endpointForm.control}
																	render={({ field, fieldState }) => (
																		<FormItem className="space-y-2">
																			<FormLabel className="text-neutral-9 text-xs">
																				Limit
																			</FormLabel>
																			<FormControl>
																				<Input
																					placeholder="e.g 10"
																					inputMode="numeric"
																					pattern="\d*"
																					type="number"
																					min={0}
																					{...field}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: ' hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																				></Input>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>
															</div>
														</div>
													)}
												</div>
												{/* Auth Section */}
												<div>
													{endpointForm.watch('showAuth') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<p className="text-xs text-neutral-11 font-medium mb-3">
																Endpoint Authentication
																{/* TODO show tooltip */}
															</p>
															<div className="grid grid-cols-2 gap-x-5">
																<FormField
																	name="authentication.api_key.header_name"
																	control={endpointForm.control}
																	render={({ field, fieldState }) => (
																		<FormItem className="space-y-2">
																			<FormLabel className="text-neutral-9 text-xs">
																				API Key Name
																			</FormLabel>
																			<FormControl>
																				<Input
																					type="text"
																					autoComplete="text"
																					{...field}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: ' hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																				></Input>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>

																<FormField
																	name="authentication.api_key.header_value"
																	control={endpointForm.control}
																	render={({ field, fieldState }) => (
																		<FormItem className="space-y-2">
																			<FormLabel className="text-neutral-9 text-xs">
																				API Key Value
																			</FormLabel>
																			<FormControl>
																				<Input
																					type="text"
																					autoComplete="text"
																					{...field}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: ' hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																				></Input>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>
															</div>
														</div>
													)}
												</div>
												{/* Notifications Section */}
												<div>
													{endpointForm.watch('showNotifications') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<p className="text-xs text-neutral-11 font-medium mb-3">
																Alert Configuration
																{/* TODO show tooltip */}
															</p>
															<div className="grid grid-cols-2 gap-x-5">
																<FormField
																	name="support_email"
																	control={endpointForm.control}
																	render={({ field, fieldState }) => (
																		<FormItem className="space-y-2">
																			<FormLabel className="text-neutral-9 text-xs">
																				Support Email (tooltip)
																			</FormLabel>
																			<FormControl>
																				<Input
																					type="email"
																					autoComplete="on"
																					disabled={
																						!licenses.includes(
																							'ADVANCED_ENDPOINT_MANAGEMENT',
																						)
																					}
																					{...field}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: ' hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																				/>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>

																<FormField
																	name="slack_webhook_url"
																	control={endpointForm.control}
																	render={({ field, fieldState }) => (
																		<FormItem className="space-y-2">
																			<FormLabel className="text-neutral-9 text-xs">
																				Slack Webhook URL (tooltip)
																			</FormLabel>
																			<FormControl>
																				<Input
																					type="url"
																					autoComplete="on"
																					disabled={
																						!licenses.includes(
																							'ADVANCED_ENDPOINT_MANAGEMENT',
																						)
																					}
																					{...field}
																					className={cn(
																						'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																						fieldState.error
																							? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																							: ' hover:border-new.primary-100 focus:border-new.primary-300',
																					)}
																				></Input>
																			</FormControl>
																			<FormMessageWithErrorIcon />
																		</FormItem>
																	)}
																/>
															</div>
														</div>
													)}
												</div>
												{/* Signature Format Section */}
												<div>
													{endpointForm.watch('showSignatureFormat') && (
														<div className="pl-4 border-l border-l-new.primary-25">
															<FormField
																control={endpointForm.control}
																name="advanced_signatures"
																render={({ field }) => (
																	<FormItem className="w-full relative mb-6 block">
																		<p className="text-xs/5 text-neutral-9 mb-2">
																			Signature Format (tooltip)
																		</p>
																		<div className="flex w-full gap-x-6">
																			{[
																				{ label: 'Simple', value: 'false' },
																				{ label: 'Advanced', value: 'true' },
																			].map(({ label, value }) => {
																				return (
																					<FormControl
																						className="w-full"
																						key={label}
																					>
																						<label
																							className={cn(
																								'cursor-pointer border border-primary-100 flex items-start gap-x-2 p-4 rounded-sm',
																								// @ts-expect-error the transformation takes care of this
																								field.value == value
																									? 'border-new.primary-300 bg-[#FAFAFE]'
																									: 'border-neutral-5',
																							)}
																							htmlFor={`sig_format_${label}`}
																						>
																							<span className="sr-only">
																								{label}
																							</span>
																							<Input
																								type="radio"
																								id={`sig_format_${label}`}
																								{...field}
																								value={value}
																								className="shadow-none h-4 w-fit"
																							/>
																							<div className="flex flex-col gap-y-1">
																								<p className="w-full text-xs text-neutral-10 font-semibold capitalize">
																									{label}
																								</p>
																							</div>
																						</label>
																					</FormControl>
																				);
																			})}
																		</div>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													)}
												</div>

												<div>
													<FormField
														name="useExistingEndpoint"
														control={endpointForm.control}
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<Button
																		disabled={!canManageSubscriptions}
																		variant="ghost"
																		type="button"
																		size="sm"
																		className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																		onClick={() => field.onChange(!field.value)}
																	>
																		Use Existing Endpoint
																	</Button>
																</FormControl>
															</FormItem>
														)}
													/>
												</div>

												<hr />

												<div className="flex gap-x-4 items-center">
													<FormField
														control={endpointForm.control}
														name="showEventsFilter"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		disabled={
																			!licenses.includes(
																				'ADVANCED_SUBSCRIPTIONS',
																			)
																		}
																		label="Events Filter"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													{project?.type == 'outgoing' && (
														<FormField
															control={endpointForm.control}
															name="showEventTypes"
															render={({ field }) => (
																<FormItem>
																	<FormControl>
																		<ConvoyCheckbox
																			disabled={
																				!licenses.includes(
																					'ADVANCED_SUBSCRIPTIONS',
																				)
																			}
																			label="Event Types"
																			isChecked={field.value}
																			onChange={field.onChange}
																		/>
																	</FormControl>
																</FormItem>
															)}
														/>
													)}

													{project?.type == 'incoming' && (
														<FormField
															control={endpointForm.control}
															name="showTransform"
															render={({ field }) => (
																<FormItem>
																	<FormControl>
																		<ConvoyCheckbox
																			disabled={
																				!licenses.includes(
																					'WEBHOOK_TRANSFORMATIONS',
																				)
																			}
																			label="Transform"
																			isChecked={field.value}
																			onChange={field.onChange}
																		/>
																	</FormControl>
																</FormItem>
															)}
														/>
													)}
												</div>

												{endpointForm.watch('showEventTypes') && (
													<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
														<FormField
															control={endpointForm.control}
															name="filter_config.event_types"
															render={({ field }) => (
																<FormItem className="w-full flex flex-col gap-y-2">
																	<div className="w-full space-y-2 flex items-center">
																		<FormLabel className="flex items-center gap-x-2">
																			<span className="text-neutral-9 text-xs/5 ">
																				Event Types
																			</span>

																			<Tooltip>
																				<TooltipTrigger
																					asChild
																					className="hover:cursor-pointer"
																				>
																					<Info
																						size={12}
																						className="ml-1 text-neutral-9 inline"
																					/>
																				</TooltipTrigger>
																				<TooltipContent className="bg-white-100 border border-neutral-4">
																					<p className="w-[300px] text-xs text-neutral-10">
																						These are the specifications for the
																						retry mechanism for your endpoints
																						under this subscription. In Linear
																						time retry, event retries are done
																						in linear time, while in Exponential
																						back off retry, events are retried
																						progressively increasing the time
																						before the next retry attempt.
																					</p>
																				</TooltipContent>
																			</Tooltip>
																		</FormLabel>
																	</div>
																	<Command className="overflow-visible">
																		<div className="rounded-md border border-input px-3 py-2 text-sm focus-within:ring-0 h-12">
																			<div className="flex flex-wrap gap-1">
																				{selectedEventTypes.map(eventType => {
																					return (
																						<Badge
																							key={eventType}
																							variant="secondary"
																							className="select-none"
																						>
																							{eventType}
																							<X
																								className="size-3 text-muted-foreground hover:text-foreground ml-2 cursor-pointer"
																								onMouseDown={e => {
																									e.preventDefault();
																								}}
																								onClick={() => {
																									handleUnselect(eventType);
																									field.onChange(
																										selectedEventTypes.filter(
																											s => s !== eventType,
																										),
																									);
																								}}
																							/>
																						</Badge>
																					);
																				})}
																				<CommandPrimitive.Input
																					onKeyDown={e => {
																						const isRemoveAction =
																							handleKeyDown(e);
																						if (isRemoveAction) {
																							field.onChange(
																								selectedEventTypes.slice(0, -1),
																							);
																						}
																					}}
																					onValueChange={setInputValue}
																					value={inputValue}
																					onBlur={() =>
																						setIsMultiSelectOpen(false)
																					}
																					onFocus={() =>
																						setIsMultiSelectOpen(true)
																					}
																					placeholder=""
																					className="ml-2 flex-1 bg-transparent outline-none placeholder:text-muted-foreground"
																				/>
																			</div>
																		</div>
																		<div className="relative mt-2">
																			<CommandList>
																				{isMultiSelectOpen &&
																					!!filteredEventTypes.length && (
																						<div className="absolute top-0 z-10 w-full rounded-md border bg-popover text-popover-foreground shadow-md outline-none">
																							<CommandGroup className="h-full overflow-auto">
																								{filteredEventTypes.map(
																									eventType => {
																										return (
																											<CommandItem
																												key={eventType}
																												onMouseDown={e => {
																													e.preventDefault();
																												}}
																												onSelect={() => {
																													setInputValue('');
																													setSelectedEventTypes(
																														prev => {
																															field.onChange([
																																...prev,
																																eventType,
																															]);
																															return [
																																...prev,
																																eventType,
																															];
																														},
																													);
																												}}
																												className={
																													'cursor-pointer'
																												}
																											>
																												{eventType}
																											</CommandItem>
																										);
																									},
																								)}
																							</CommandGroup>
																						</div>
																					)}
																			</CommandList>
																		</div>
																	</Command>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												<div className="flex flex-col gap-y-6">
													{endpointForm.watch('showEventsFilter') && (
														<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
															<div className="flex flex-col gap-y-2 justify-center">
																<p className="text-neutral-10 font-medium text-xs">
																	Events filter
																</p>
																<p className="text-[10px] text-neutral-10">
																	Filter events received by request body and
																	header
																</p>
															</div>
															<div>
																<Button
																	type="button"
																	variant="outline"
																	size="sm"
																	disabled={
																		!licenses.includes('ADVANCED_SUBSCRIPTIONS')
																	}
																	className="text-xs text-neutral-10 shadow-none hover:text-neutral-10 hover:bg-white-100"
																	onClick={e => {
																		e.stopPropagation();
																		setShowEventsFilterDialog(true);
																	}}
																>
																	Open Editor
																</Button>
															</div>
														</div>
													)}

													{endpointForm.watch('showTransform') && (
														<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
															<div className="flex flex-col gap-y-2 justify-center">
																<p className="text-neutral-10 font-medium text-xs">
																	Transform
																</p>
																<p className="text-[10px] text-neutral-10">
																	Transform request body of events with a
																	JavaScript function.
																</p>
															</div>
															<div>
																<Button
																	type="button"
																	variant="outline"
																	size="sm"
																	disabled={
																		!licenses.includes(
																			'WEBHOOK_TRANSFORMATIONS',
																		)
																	}
																	className="text-xs text-neutral-10 shadow-none hover:text-neutral-10 hover:bg-white-100"
																	onClick={e => {
																		e.stopPropagation();
																		setShowTransformFunctionDialog(true);
																	}}
																>
																	Open Editor
																</Button>
															</div>
														</div>
													)}
												</div>
											</div>
										)}
									</form>
								</Form>
							</div>
						</section>
					</div>
					<SheetFooter className="flex justify-end items-center gap-x-2 pt-6 pr-4">
						<SheetClose asChild>
							<Button
								size={'sm'}
								variant={'outline'}
								className="border border-destructive text-destructive hover:text-destructive hover:bg-white-100 shadow-none"
								onClick={() => {
									setIsViewDetailsDialogOpen(false);
									setIsDeleteDialogOpen(true);
								}}
							>
								Delete
							</Button>
						</SheetClose>
						<SheetClose asChild>
							<Button
								size={'sm'}
								variant={'ghost'}
								className="text-white-100 hover:text-white-100 bg-new.primary-400 hover:bg-new.primary-400 shadow-none"
								onClick={() =>
									navigate({
										to: '/projects/$projectId/subscriptions/$subscriptionId',
										params: {
											projectId,
											subscriptionId: currentSub?.uid as string,
										},
									})
								}
							>
								Edit
							</Button>
						</SheetClose>
					</SheetFooter>
				</SheetContent>
			</Sheet>

			{/* Delete Subscription Dialog */}
			<Dialog
				open={isDeleteDialogOpen}
				onOpenChange={() => setIsDeleteDialogOpen(!isDeleteDialogOpen)}
			>
				<DialogContent className="sm:max-w-[432px] rounded-lg">
					<DialogHeader>
						<DialogTitle className="flex justify-center items-center">
							<img src={warningAnimation} alt="warning" className="w-24" />
						</DialogTitle>
						<DialogDescription className="flex justify-center items-center font-medium text-new.black text-sm">
							Are you sure you want to delete &quot;
							{currentSub?.name}&quot;?
						</DialogDescription>
					</DialogHeader>
					<div className="flex flex-col items-center space-y-4">
						<p className="text-xs text-neutral-11">
							This action is irreversible.
						</p>
						<DialogClose asChild>
							<Button
								onClick={async () =>
									await deleteSubscription(currentSub?.uid as string)
								}
								disabled={isDeletingSub || !canManageSubscriptions}
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

			{/* Events Filter Dialog New*/}
			<Dialog
				open={showEventsFilterDialog}
				onOpenChange={setShowEventsFilterDialog}
			>
				<DialogContent className="w-[90%] h-[90%] max-w-[90%] max-h-[90%] p-0 overflow-hidden rounded-8px gap-0">
					<div className="flex flex-col h-full">
						{/* Dialog Header */}
						<DialogHeader className="px-12 py-4 border-b border-neutral-4">
							<div className="flex items-center justify-between w-full">
								<DialogTitle className="text-base font-semibold">
									Subscription Filter
								</DialogTitle>

								<DialogDescription className="sr-only">
									Subscription Filter
								</DialogDescription>

								<div className="flex items-center justify-end gap-x-4">
									<Button
										variant="outline"
										size="sm"
										className="shadow-none border-new.primary-400 hover:bg-white-100 text-new.primary-400 hover:text-new.primary-400"
										onClick={testFilter}
									>
										Test Filter
										<svg width="18" height="18" className="fill-white-100">
											<use xlinkHref="#test-icon"></use>
										</svg>
									</Button>
									<Button
										size="sm"
										variant="ghost"
										className="shadow-none bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100"
										disabled={!hasPassedTestFilter}
										onClick={setSubscriptionFilter}
									>
										Save
									</Button>
								</div>
							</div>
						</DialogHeader>
					</div>
					<div className="flex-1 overflow-auto px-6">
						<div className="min-w-[80vw] mx-auto">
							{/* Tabs for Body and Header*/}
							<Tabs defaultValue="body" className="w-full">
								<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-center">
									<TabsTrigger
										value="body"
										className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
									>
										<span className="text-sm">Body</span>
									</TabsTrigger>
									<TabsTrigger
										value="header"
										className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
									>
										<span className="text-sm">Header</span>
									</TabsTrigger>
								</TabsList>

								{/* Body Tab */}
								<TabsContent value="body">
									<div className="flex w-full border border-neutral-4 rounded-lg">
										{/* Body Event Payload */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Input */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Event Payload
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.request.body}
														onChange={body =>
															setEventsFilter(prev => ({
																...prev,
																request: { ...prev.request, body: body || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>

										{/* Body Schema */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Filter Schema */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Filter Schema
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.schema.body}
														onChange={body =>
															setEventsFilter(prev => ({
																...prev,
																schema: { ...prev.schema, body: body || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>
									</div>
								</TabsContent>

								<TabsContent value="header">
									<div className="flex w-full border border-neutral-4 rounded-lg">
										{/* Body Event Payload */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Input */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Event Payload
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.request.header}
														onChange={header =>
															setEventsFilter(prev => ({
																...prev,
																request: {
																	...prev.request,
																	header: header || '',
																},
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>

										{/* Body Schema */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Filter Schema */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Filter Schema
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.schema.header}
														onChange={header =>
															setEventsFilter(prev => ({
																...prev,
																schema: {
																	...prev.schema,
																	header: header || '',
																},
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>
									</div>
								</TabsContent>
							</Tabs>
						</div>
					</div>
				</DialogContent>
			</Dialog>

			{/* Transform Function Dialog */}
			<Dialog
				open={showTransformFunctionDialog}
				onOpenChange={open => {
					if (!open && !hasSavedFn) {
						alert('You have not saved your function'); // TODO use toast instead
						setShowTransformFunctionDialog(false);
						return;
					}
					if (!open) setShowTransformFunctionDialog(false);
				}}
			>
				<DialogContent className="w-[90%] h-[90%] max-w-[90%] max-h-[90%] p-0 overflow-hidden rounded-8px gap-0">
					<div className="flex flex-col h-full">
						{/* Dialog Header */}
						<DialogHeader className="px-12 py-4 border-b border-neutral-4">
							<div className="flex items-center justify-between w-full">
								<DialogTitle className="text-base font-semibold">
									Source Transform
								</DialogTitle>

								<DialogDescription className="sr-only">
									Source Transform
								</DialogDescription>
								<Button
									variant="ghost"
									size={'sm'}
									className="px-4 py-2 bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100 text-xs"
									onClick={() => {
										setHasSavedFn(true);
										setShowTransformFunctionDialog(false);
										saveTransformFn();
									}}
									disabled={!isTransformPassed}
								>
									<Save className="stroke-white-100" />
									Save Function
								</Button>
							</div>
						</DialogHeader>

						{/* Dialog Body */}
						<div className="flex-1 overflow-auto px-6">
							<div className="min-w-[80vw] mx-auto">
								{/* Tabs for Body/Header */}
								<Tabs defaultValue="body" className="w-full">
									<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-center">
										<TabsTrigger
											value="body"
											className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
										>
											<span className="text-sm">Body</span>
										</TabsTrigger>
										<TabsTrigger
											value="header"
											className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
										>
											<span className="text-sm">Header</span>
										</TabsTrigger>
									</TabsList>
									{/* Body Tab*/}
									<TabsContent value="body">
										<div className="flex w-full border border-neutral-4 rounded-lg">
											<div className="flex flex-col w-1/2 border-r border-neutral-4">
												<div>
													{/* Body Input */}
													<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
														Input
													</div>
													<div className="h-[300px]">
														<Editor
															height="300px"
															language="json"
															value={JSON.stringify(
																transformBodyPayload,
																null,
																2,
															)}
															onChange={value =>
																setTransformBodyPayload(
																	JSON.parse(value || '{}'),
																)
															}
															options={editorOptions}
														/>
													</div>
												</div>
												<div className="min-h-[370px]">
													<Tabs defaultValue="output">
														<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-start">
															<TabsTrigger
																value="output"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Output</span>
															</TabsTrigger>
															<TabsTrigger
																value="diff"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Diff</span>
															</TabsTrigger>
														</TabsList>
														<div className="h-[336px]">
															{/* Body Output */}
															<TabsContent value="output" className="h-[336px]">
																<Editor
																	height="336px"
																	language="json"
																	value={
																		typeof bodyOutput.current === 'object'
																			? JSON.stringify(
																					bodyOutput.current,
																					null,
																					2,
																				)
																			: `${bodyOutput.current}`
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</TabsContent>
															{/* Body Diff */}
															<TabsContent value="diff" className="h-[336px]">
																<DiffEditor
																	height="336px"
																	language="json"
																	original={
																		typeof bodyOutput.previous === 'object'
																			? JSON.stringify(
																					bodyOutput.previous,
																					null,
																					2,
																				)
																			: `${bodyOutput.previous}`
																	}
																	modified={
																		// TODO change to body diff
																		typeof bodyOutput.current === 'object'
																			? JSON.stringify(
																					bodyOutput.current,
																					null,
																					2,
																				)
																			: `${bodyOutput.current}`
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</TabsContent>
														</div>
													</Tabs>
												</div>
											</div>
											<div className="flex flex-col w-1/2">
												<div className="flex justify-between items-center text-xs  border-b border-neutral-4 px-3 py-2 rounded-tr-lg">
													<span className="text-neutral-11">
														Transform Function
													</span>
													<Button
														variant="outline"
														size="sm"
														className="h-6 py-0 px-2 text-xs border border-new.primary-300 text-new.primary-300 gap-1 hover:text-new.primary-300 hover:bg-white-100 shadow-none"
														disabled={isTestingFunction}
														onClick={() => {
															setShowConsole(true);
															testTransformFunction('body');
														}}
													>
														<svg
															width="10"
															height="11"
															viewBox="0 0 10 11"
															fill="none"
															xmlns="http://www.w3.org/2000/svg"
															className=""
														>
															<path
																d="M1.66797 5.5004V4.01707C1.66797 2.1754 2.97214 1.42124 4.56797 2.34207L5.85547 3.08374L7.14297 3.8254C8.7388 4.74624 8.7388 6.25457 7.14297 7.1754L5.85547 7.91707L4.56797 8.65874C2.97214 9.57957 1.66797 8.8254 1.66797 6.98374V5.5004Z"
																stroke="#477DB3"
																strokeMiterlimit="10"
																strokeLinecap="round"
																strokeLinejoin="round"
															/>
														</svg>
														Run
													</Button>
												</div>
												{/* Body Transform Function */}
												<div
													className={showConsole ? 'h-[500px]' : 'h-[632px]'}
												>
													<Editor
														height="100%"
														language="javascript"
														value={transformFnBody}
														onChange={value => {
															setTransformFnBody(value || '');
															setHasSavedFn(false);
														}}
														options={editorOptions}
													/>
												</div>

												{/* Body Console */}
												{(showConsole || bodyLogs.length > 0) && (
													<div className="border-t border-neutral-4">
														<div className="flex justify-between items-center px-3 py-1.5 text-xs text-neutral-11">
															<span>Console</span>
															<Button
																variant="ghost"
																size="sm"
																className="h-5 w-5 p-0"
																onClick={() => setShowConsole(false)}
															>
																<svg
																	width="14"
																	height="14"
																	className="fill-neutral-10"
																>
																	<use xlinkHref="#close-icon"></use>
																</svg>
															</Button>
														</div>
														<div className="h-[132px] bg-neutral-1 p-2 overflow-auto">
															{bodyLogs.map((log, index) => (
																<div
																	key={index}
																	className="text-xs font-mono whitespace-pre-wrap"
																>
																	{log}
																</div>
															))}
														</div>
													</div>
												)}
											</div>
										</div>
									</TabsContent>

									{/* Header Tab */}
									<TabsContent value="header">
										<div className="flex w-full border border-neutral-4 rounded-lg">
											<div className="flex flex-col w-1/2 border-r border-neutral-4">
												<div>
													<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
														Input
													</div>
													{/* Header Input */}
													<div className="h-[300px]">
														<Editor
															height="300px"
															language="json"
															value={JSON.stringify(headerPayload, null, 2)}
															onChange={value =>
																setHeaderPayload(JSON.parse(value || '{}'))
															}
															options={{ ...editorOptions, readOnly: false }}
														/>
													</div>
												</div>
												<div className="min-h-[370px]">
													<Tabs defaultValue="output">
														<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-start">
															<TabsTrigger
																value="output"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Output</span>
															</TabsTrigger>
															<TabsTrigger
																value="diff"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Diff</span>
															</TabsTrigger>
														</TabsList>
														<TabsContent value="output">
															<div className="h-[336px]">
																<Editor
																	height="336px"
																	language="json"
																	value={
																		typeof headerOutput.current === 'object'
																			? JSON.stringify(
																					headerOutput.current,
																					null,
																					2,
																				)
																			: String(headerOutput.current)
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</div>
														</TabsContent>
														<TabsContent value="diff">
															<DiffEditor
																height="336px"
																language="json"
																original={
																	typeof headerOutput.previous === 'object'
																		? JSON.stringify(
																				headerOutput.previous,
																				null,
																				2,
																			)
																		: String(headerOutput.previous)
																}
																modified={
																	typeof headerOutput.current === 'object'
																		? JSON.stringify(
																				headerOutput.current,
																				null,
																				2,
																			)
																		: String(headerOutput.current)
																}
																options={{ ...editorOptions, readOnly: true }}
															/>
														</TabsContent>
													</Tabs>
												</div>
											</div>
											<div className="flex flex-col w-1/2">
												<div className="flex justify-between items-center text-xs text-neutral-11 border-b border-neutral-4 px-3 py-2 rounded-tr-lg">
													<span>Transform Function</span>
													<Button
														variant="outline"
														size="sm"
														disabled={isTestingFunction}
														className="h-6 py-0 px-2 text-xs border border-new.primary-300 text-new.primary-300 gap-1 hover:text-new.primary-300 hover:bg-white-100 shadow-none"
														onClick={() => testTransformFunction('header')}
													>
														<svg
															width="10"
															height="11"
															viewBox="0 0 10 11"
															fill="none"
															xmlns="http://www.w3.org/2000/svg"
															className=""
														>
															<path
																d="M1.66797 5.5004V4.01707C1.66797 2.1754 2.97214 1.42124 4.56797 2.34207L5.85547 3.08374L7.14297 3.8254C8.7388 4.74624 8.7388 6.25457 7.14297 7.1754L5.85547 7.91707L4.56797 8.65874C2.97214 9.57957 1.66797 8.8254 1.66797 6.98374V5.5004Z"
																stroke="#477DB3"
																strokeMiterlimit="10"
																strokeLinecap="round"
																strokeLinejoin="round"
															/>
														</svg>
														Run
													</Button>
												</div>
												{/* Header Transform Function*/}
												<div
													className={showConsole ? 'h-[500px]' : 'h-[632px]'}
												>
													<Editor
														height="100%"
														language="javascript"
														value={transformFnHeader}
														onChange={value => {
															setTransformFnHeader(value || '');
															setHasSavedFn(false);
														}}
														options={editorOptions}
													/>
												</div>

												{showConsole && headerLogs.length > 0 && (
													<div className="border-t border-neutral-4">
														<div className="flex justify-between items-center px-3 py-2 text-xs text-neutral-11">
															<span>Console</span>
															<Button
																variant="ghost"
																size="sm"
																className="h-5 w-5 p-0"
																onClick={() => setShowConsole(false)}
															>
																<svg
																	width="14"
																	height="14"
																	className="fill-neutral-10"
																>
																	<use xlinkHref="#close-icon"></use>
																</svg>
															</Button>
														</div>
														<div className="h-[132px] bg-neutral-1 p-2 overflow-auto">
															{headerLogs.map((log, index) => (
																<div
																	key={index}
																	className="text-xs font-mono whitespace-pre-wrap"
																>
																	{log}
																</div>
															))}
														</div>
													</div>
												)}
											</div>
										</div>
									</TabsContent>
								</Tabs>
							</div>
						</div>
					</div>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
