import { z } from 'zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';

import { ChevronRight, Info, CopyIcon } from 'lucide-react';

import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { InputTags } from '@/components/ui/input-tags';
import { Textarea } from '@/components/ui/textarea';
import { ConvoyCheckbox } from '@/components/convoy-checkbox';
import { ToggleGroupItem } from '@/components/ui/toggle-group';
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from '@/components/ui/tooltip';
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogFooter,
} from '@/components/ui/dialog';
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from '@/components/ui/select';
import { ToggleGroup } from '@radix-ui/react-toggle-group';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';

import { cn } from '@/lib/utils';
import { useLicenseStore, useProjectStore } from '@/store';
import * as authService from '@/services/auth.service';
import * as sourcesService from '@/services/sources.service';

import githubIcon from '../../../../../assets/img/github.png';
import shopifyIcon from '../../../../../assets/img/shopify.png';
import twitterIcon from '../../../../../assets/img/twitter.png';
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

export const Route = createFileRoute('/projects_/$projectId/sources/new')({
	component: RouteComponent,
	async loader() {
		const perms = await authService.getUserPermissions();
		return {
			canManageSources: perms.includes('Sources|MANAGE'),
		};
	},
});

const CreateIncomingSourceFormSchema = z
	.object({
		name: z.string().min(1, 'Enter new source name'),
		type: z.enum([
			'noop',
			'hmac',
			'basic_auth',
			'api_key',
			'github',
			'shopify',
			'twitter',
		]),
		// is_disabled: z.boolean().optional().default(false),
		config: z.object({
			// TODO: In refine, encoding, hash, header and secret are required if type == hmac
			encoding: z.enum(['base64', 'hex', '']).optional(),
			hash: z.enum(['SHA256', 'SHA512', '']).optional(),
			header: z.string().optional(),
			secret: z.string().optional(),
			username: z.string().optional(),
			password: z.string().optional(),
			header_name: z.string().optional(),
			header_value: z.string().optional(),
		}), // z.record(z.union([z.string(), z.boolean()])).optional(),
		custom_response: z
			.object({
				content_type: z.string().optional(),
				body: z.string().optional(),
			})
			.optional(),
		idempotency_keys: z.array(z.string()).optional(),
		showHmac: z.boolean(),
		showBasicAuth: z.boolean(),
		showAPIKey: z.boolean(),
		showGithub: z.boolean(),
		showTwitter: z.boolean(),
		showShopify: z.boolean(),
		showCustomResponse: z.boolean(),
		showIdempotency: z.boolean(),
	})
	.refine(
		v => {
			if (
				v.showCustomResponse &&
				(!v.custom_response?.content_type || !v.custom_response.body)
			) {
				return false;
			}
			return true;
		},
		({ custom_response }) => {
			if (!custom_response?.content_type)
				return {
					message: 'Enter content type',
					path: ['custom_response.content_type'],
				};

			if (!custom_response?.body)
				return {
					message: 'Enter response content',
					path: ['custom_response.body'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ showIdempotency, idempotency_keys }) => {
			if (showIdempotency && idempotency_keys?.length == 0) return false;
			return true;
		},
		() => {
			return {
				message:
					'Add at least one idempotency key if idempotency configuration is enabled',
				path: ['idempotency_keys'],
			};
		},
	)
	.refine(
		({ type, config }) => {
			const { encoding, hash, header, secret } = config;
			const hasInvalidValue = !encoding || !hash || !header || !secret;
			if (type == 'hmac' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { encoding, hash, header, secret } = config;
			if (!encoding)
				return {
					message: 'Enter encoding value',
					path: ['config.encoding'],
				};

			if (!hash)
				return {
					message: 'Enter hash value',
					path: ['config.hash'],
				};

			if (!header)
				return {
					message: 'Enter header key',
					path: ['config.header'],
				};

			if (!secret)
				return {
					message: 'Enter webhook signing secret',
					path: ['config.secret'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ type, config }) => {
			const { secret } = config;
			const isPreconfigured = ['github', 'shopify', 'twitter'].includes(type);
			if (isPreconfigured && !secret) return false;
			return true;
		},
		() => ({
			message: 'Enter webhook signing secret',
			path: ['config.secret'],
		}),
	)
	.refine(
		({ type, config }) => {
			const { username, password } = config;
			const hasInvalidValue = !username || !password;
			if (type == 'basic_auth' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { username, password } = config;
			if (!username)
				return {
					message: 'Enter username',
					path: ['config.username'],
				};

			if (!password)
				return {
					message: 'Enter password',
					path: ['config.password'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ type, config }) => {
			const { header_name, header_value } = config;
			const hasInvalidValue = !header_name || !header_value;
			if (type == 'api_key' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { header_name, header_value } = config;
			if (!header_name)
				return {
					message: 'Enter API header key',
					path: ['config.header_name'],
				};

			if (!header_value)
				return {
					message: 'Enter API header value',
					path: ['config.header_value'],
				};

			return { message: '', path: [] };
		},
	);

const CreateOutgoingSourceFormSchema = z.object({
	name: z.string({ required_error: 'Enter new source name' }),
	type: z.enum(['google', 'kafka', 'sqs', 'amqp', '']),
	verifier: z.object({
		type: z.enum([
			'noop',
			'hmac',
			'basic_auth',
			'api_key',
			'github',
			'shopify',
			'twitter',
		]),
	}),
	workers: z.number({ required_error: 'Enter number of workers' }).int(),
});

function RouteComponent() {
	const navigate = useNavigate()
	const { project } = useProjectStore();
	const { licenses } = useLicenseStore();
	const { projectId } = Route.useParams();
	const [isCreating, setIsCreating] = useState(false);
	const [hasCreatedIncomingSource, setHasCreatedIncomingSource] =
		useState(false);
	const [sourceUrl, setSourceUrl] = useState('');

	const pubSubTypes = [
		{ uid: 'google', name: 'Google Pub/Sub' },
		{ uid: 'kafka', name: 'Kafka' },
		{ uid: 'sqs', name: 'AWS SQS' },
		{ uid: 'amqp', name: 'AMQP / RabbitMQ' },
	];

	const sourceVerifications = [
		{ uid: 'noop', name: 'None' },
		{ uid: 'hmac', name: 'HMAC' },
		{ uid: 'basic_auth', name: 'Basic Auth' },
		{ uid: 'api_key', name: 'API Key' },
		{ uid: 'github', name: 'Github' },
		{ uid: 'shopify', name: 'Shopify' },
		{ uid: 'twitter', name: 'Twitter' },
	] as const;

	type SourceType = (typeof sourceVerifications)[number]['uid'];

	const createIncomingSourceForm = useForm({
		resolver: zodResolver(CreateIncomingSourceFormSchema),
		defaultValues: {
			name: '',
			type: 'noop',
			config: {
				hash: '',
				encoding: '',
				header: '',
				secret: '',
				username: '',
				password: '',
				header_name: '',
				header_value: '',
			},
			custom_response: {
				content_type: '',
				body: '',
			},
			idempotency_keys: [],
			showHmac: false,
			showBasicAuth: false,
			showAPIKey: false,
			showGithub: false,
			showTwitter: false,
			showShopify: false,
			showCustomResponse: false,
			showIdempotency: false,
		},
		mode: 'onTouched',
	});

	function getVerifierType(
		type: SourceType,
		config: z.infer<typeof CreateIncomingSourceFormSchema>['config'],
	) {
		const obj: Record<string, string> = {};

		if (type == 'hmac') {
			return {
				type: 'hmac',
				// @ts-expect-error types match
				hmac: Object.entries(config).reduce((acc, record: [string, string]) => {
					const [key, val] = record;
					if (['encoding', 'hash', 'header', 'secret'].includes(key)) {
						return (acc[key] = val);
					}
					return acc;
				}, obj),
			};
		}

		if (type == 'basic_auth') {
			return {
				type: 'basic_auth',
				basic_auth: Object.entries(config).reduce(
					// @ts-expect-error types match
					(acc, record: [string, string]) => {
						const [key, val] = record;
						if (['password', 'username'].includes(key)) {
							return (acc[key] = val);
						}
						return acc;
					},
					obj,
				),
			};
		}

		if (type == 'api_key') {
			return {
				type: 'api_key',
				api_key: Object.entries(config).reduce(
					// @ts-expect-error types match
					(acc, record: [string, string]) => {
						const [key, val] = record;
						if (['header_name', 'header_value'].includes(key)) {
							return (acc[key] = val);
						}
						return acc;
					},
					obj,
				),
			};
		}

		if (['github', 'shopify', 'twitter'].includes(type)) {
			return {
				type: 'hmac',
				hmac: {
					encoding: type == 'github' ? 'hex' : 'base64',
					hash: 'SHA256',
					header: `X-${type == 'github' ? 'Hub' : type == 'shopify' ? 'Shopify-Hmac' : 'Twitter-Webhooks'}-Signature-256`,
					secret: config.secret,
				},
			};
		}

		return {
			type: 'noop',
			noop: obj,
		};
	}

	function transformIncomingSource(
		v: z.infer<typeof CreateIncomingSourceFormSchema>,
	) {
		return {
			name: v.name,
			is_disabled: true,
			type: 'http',
			custom_response: {
				body: v.custom_response?.body || '',
				content_type: v.custom_response?.content_type || '',
			},
			idempotency_keys: v.idempotency_keys?.length ? v.idempotency_keys : null,
			verifier: getVerifierType(v.type, v.config),
			provider: ['github', 'twitter', 'shopify'].includes(v.type) ? v.type : '',
		};
	}

	async function saveIncomingSource(
		raw: z.infer<typeof CreateIncomingSourceFormSchema>,
	) {
		const payload = transformIncomingSource(raw);
		setIsCreating(true);
		try {
			const response = await sourcesService.createSource(payload);
			setSourceUrl(response.url);
			setHasCreatedIncomingSource(true);
		} catch (error) {
			console.error(error);
		} finally {
			setIsCreating(false);
		}
	}

	const createOutgoingSourceForm = useForm({
		resolver: zodResolver(CreateOutgoingSourceFormSchema),
		mode: 'onTouched',
	});

	async function saveOutgoingSource(raw: any) {
		console.log(raw);
	}

	return (
		<DashboardLayout showSidebar={false}>
			<section className="flex flex-col p-2 max-w-[770px] min-w-[600px] w-full m-auto my-4 gap-y-6">
				<div className="flex justify-start items-center gap-2">
					<Link
						to="/projects/$projectId/sources"
						params={{ projectId }}
						className="flex justify-center items-center p-2 bg-new.primary-25 rounded-8px"
						activeProps={{}}
					>
						<img
							src={modalCloseIcon}
							alt="back to endpoints"
							className="h-3 w-3"
						/>
					</Link>
					<h1 className="font-semibold text-sm">Create Source</h1>
				</div>

				{project?.type == 'incoming' && (
					<Form {...createIncomingSourceForm}>
						<form
							onSubmit={createIncomingSourceForm.handleSubmit(
								// @ts-expect-error it works. trust me bro
								saveIncomingSource,
							)}
							className="w-full"
						>
							<div className="border border-neutral-4 rounded-8px p-6 relative w-full">
								<div className="grid grid-cols-1 w-full gap-y-5">
									<h3 className="font-semibold mb-5 text-xs text-neutral-10">
										Pre-configured Sources
									</h3>
									<div className="flex flex-col gap-y-2">
										<ToggleGroup
											type="single"
											className="flex justify-start items-center gap-x-4"
											value={createIncomingSourceForm.watch('type')}
											onValueChange={(v: SourceType) => {
												createIncomingSourceForm.setValue('type', v);
												createIncomingSourceForm.setValue(
													'name',
													`${v.charAt(0).toUpperCase()}${v.slice(1)} Source`,
												);
											}}
										>
											<ToggleGroupItem
												value="github"
												aria-label="Toggle github"
												className={cn(
													'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
													createIncomingSourceForm.watch('type') === 'github' &&
														'border-new.primary-400',
												)}
											>
												<img
													src={githubIcon}
													alt="github preconfigured source"
												/>
											</ToggleGroupItem>
											<ToggleGroupItem
												value="shopify"
												aria-label="Toggle shopify"
												className={cn(
													'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
													createIncomingSourceForm.watch('type') ===
														'shopify' && 'border-new.primary-400',
												)}
											>
												<img
													src={shopifyIcon}
													alt="shopify preconfigured source"
												/>
											</ToggleGroupItem>
											<ToggleGroupItem
												value="twitter"
												aria-label="Toggle twitter"
												className={cn(
													'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
													createIncomingSourceForm.watch('type') ===
														'twitter' && 'border-new.primary-400',
												)}
											>
												<img
													src={twitterIcon}
													alt="twitter preconfigured source"
												/>
											</ToggleGroupItem>
										</ToggleGroup>
									</div>
									<hr />
									<div>
										<FormField
											name="name"
											control={createIncomingSourceForm.control}
											render={({ field, fieldState }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Source Name
													</FormLabel>
													<FormControl>
														<Input
															type="text"
															autoComplete="text"
															placeholder="Enter source name"
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
									<div>
										<FormField
											name="type"
											control={createIncomingSourceForm.control}
											render={({ field }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Source Verification
													</FormLabel>
													<Select
														value={createIncomingSourceForm.watch('type')}
														onValueChange={(v: SourceType) => {
															field.onChange(v);
															if (
																['github', 'shopify', 'twitter'].includes(v)
															) {
																createIncomingSourceForm.setValue(
																	'name',
																	`${v.charAt(0).toUpperCase()}${v.slice(1)} Source`,
																);
															}
														}}
														defaultValue={field.value}
													>
														<FormControl>
															<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																<SelectValue
																	placeholder=""
																	className="text-xs text-neutral-10"
																/>
															</SelectTrigger>
														</FormControl>
														<SelectContent className="shadow-none">
															{sourceVerifications.map(sv => (
																<SelectItem
																	className="cursor-pointer text-xs py-3 hover:bg-transparent"
																	value={sv.uid}
																	key={sv.uid}
																>
																	<span className="text-neutral-10">
																		{sv.name}
																	</span>
																</SelectItem>
															))}
														</SelectContent>
													</Select>
												</FormItem>
											)}
										/>
									</div>

									{/* When source verification is HMAC */}
									{createIncomingSourceForm.watch('type') == 'hmac' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure HMAC
											</p>

											<FormField
												name="config.encoding"
												control={createIncomingSourceForm.control}
												render={({ field }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Encoding
														</FormLabel>
														<Select
															onValueChange={field.onChange}
															defaultValue={field.value}
														>
															<FormControl>
																<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																	<SelectValue
																		placeholder=""
																		className="text-xs text-neutral-10"
																	/>
																</SelectTrigger>
															</FormControl>
															<SelectContent className="shadow-none">
																{['base64', 'hex'].map(encoding => (
																	<SelectItem
																		className="cursor-pointer text-xs py-3 hover:bg-transparent"
																		value={encoding}
																		key={encoding}
																	>
																		<span className="text-neutral-10">
																			{encoding}
																		</span>
																	</SelectItem>
																))}
															</SelectContent>
														</Select>
													</FormItem>
												)}
											/>

											<FormField
												name="config.hash"
												control={createIncomingSourceForm.control}
												render={({ field }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Hash Algorithm
														</FormLabel>
														<Select
															onValueChange={field.onChange}
															defaultValue={field.value}
														>
															<FormControl>
																<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																	<SelectValue
																		placeholder=""
																		className="text-xs text-neutral-10"
																	/>
																</SelectTrigger>
															</FormControl>
															<SelectContent className="shadow-none">
																{['SHA256', 'SHA512'].map(hash => (
																	<SelectItem
																		className="cursor-pointer text-xs py-3 hover:bg-transparent"
																		value={hash}
																		key={hash}
																	>
																		{' '}
																		<span className="text-neutral-10">
																			{hash}
																		</span>
																	</SelectItem>
																))}
															</SelectContent>
														</Select>
													</FormItem>
												)}
											/>

											<FormField
												name="config.header"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Header Value
														</FormLabel>
														<FormControl>
															<Input
																type="text"
																autoComplete="text"
																placeholder="Key goes here"
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
												name="config.secret"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Webhook signing secret
														</FormLabel>
														<FormControl>
															<Input
																type="text"
																placeholder="Secret goes here"
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

									{/* When source verification is basic auth */}
									{createIncomingSourceForm.watch('type') == 'basic_auth' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure Basic Auth
											</p>

											<FormField
												name="config.username"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Username
														</FormLabel>
														<FormControl>
															<Input
																type="text"
																autoComplete="off"
																placeholder="Username goes here"
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
												name="config.password"
												control={createOutgoingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Password
														</FormLabel>
														<FormControl>
															<Input
																type="password"
																autoComplete="off"
																placeholder="********"
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
									)}

									{/* When source verification is API Key */}
									{createIncomingSourceForm.watch('type') == 'api_key' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure API Key
											</p>

											<FormField
												name="config.header_name"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Header Name
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
												name="config.header_value"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Header Value
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
															/>
														</FormControl>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>
										</div>
									)}

									{/* When source verification is github, twitter or shopify */}
									{['github', 'shopify', 'twitter'].includes(
										createIncomingSourceForm.watch('type'),
									) && (
										<div className="grid grid-cols-1 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												{createIncomingSourceForm.watch('type')} Credentials
											</p>

											<FormField
												name="config.secret"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Webhook Signing Secret
														</FormLabel>
														<FormControl>
															<Input
																type="text"
																placeholder="Secret goes here"
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

									<div className="py-3">
										<hr />
									</div>

									{/* Checkboxes for custom response and idempotency */}
									<div className="flex items-center gap-x-4">
										<FormField
											control={createIncomingSourceForm.control}
											name="showCustomResponse"
											render={({ field }) => (
												<FormItem>
													<FormControl>
														<ConvoyCheckbox
															label="Custom Response"
															isChecked={field.value}
															onChange={field.onChange}
														/>
													</FormControl>
												</FormItem>
											)}
										/>

										<FormField
											control={createIncomingSourceForm.control}
											name="showIdempotency"
											render={({ field }) => (
												<FormItem>
													<FormControl>
														<ConvoyCheckbox
															label="Idempotency"
															isChecked={field.value}
															onChange={field.onChange}
														/>
													</FormControl>
												</FormItem>
											)}
										/>
									</div>

									{/* Custom Response */}
									{createIncomingSourceForm.watch('showCustomResponse') && (
										<div className="border-l-2 border-new.primary-25 pl-4 flex flex-col gap-y-4">
											<h3 className="text-xs text-neutral-10 font-semibold">
												Custom Response
											</h3>

											<FormField
												name="custom_response.content_type"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Response Content Type
														</FormLabel>
														<FormControl>
															<Input
																type="text"
																placeholder="application/json, text/plain"
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
												name="custom_response.body"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Response Content
														</FormLabel>
														<FormControl>
															<Textarea
																rows={6}
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
									)}

									{/* Idempotency */}
									{createIncomingSourceForm.watch('showIdempotency') && (
										<div className="border-l-2 border-new.primary-25 pl-4 flex flex-col gap-y-4">
											<h3 className="text-xs text-neutral-10 font-semibold">
												Idempotency Config
											</h3>

											<FormField
												name="idempotency_keys"
												control={createIncomingSourceForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="flex items-center gap-x-1">
															<span className="text-neutral-9 text-xs">
																Key locations
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
																		JSON location of idempotency key, add
																		multiple locations for different locations
																	</p>
																</TooltipContent>
															</Tooltip>
														</FormLabel>
														<FormControl>
															<InputTags
																className={cn(
																	'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																	fieldState.error
																		? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																		: ' hover:border-new.primary-100 focus:border-new.primary-300',
																)}
																name={field.name}
																onChange={field.onChange}
																value={field.value || []}
															/>
														</FormControl>
														<p className="text-[10px] text-neutral-10 italic">
															The order matters. Set the value of each input
															with a coma (,)
														</p>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>
										</div>
									)}
								</div>
							</div>
							{/* Submit Button */}
							<div className="flex justify-end mt-6 w-full">
								<Button
									type="submit"
									disabled={
										!createIncomingSourceForm.formState.isValid ||
										isCreating ||
										!licenses.includes('WEBHOOK_TRANSFORMATIONS')
									}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isCreating ? 'Creating...' : 'Create'} Source
									<ChevronRight className="stroke-white-100" />
								</Button>
							</div>
						</form>
					</Form>
				)}

				{project?.type == 'outgoing' && (
					<Form {...createOutgoingSourceForm}>
						<form
							onSubmit={createOutgoingSourceForm.handleSubmit(
								saveOutgoingSource,
							)}
							className="w-full"
						>
							<div className="border border-neutral-4 rounded-8px p-6 relative w-full">
								<div className="grid grid-cols-1 w-full  gap-y-5">
									<div>
										<FormField
											control={createOutgoingSourceForm.control}
											name="name"
											render={({ field, fieldState }) => (
												<FormItem className="w-full relative block">
													<div className="w-full mb-2 flex items-center justify-between">
														<FormLabel className="text-xs/5 text-neutral-9">
															Source name
														</FormLabel>
													</div>
													<FormControl>
														<Input
															autoComplete="on"
															type="text"
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
															placeholder="Enter source name"
															{...field}
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>
									</div>
									<div>
										<FormField
											name="type"
											control={createOutgoingSourceForm.control}
											render={({ field }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Source Verification
													</FormLabel>
													<Select
														onValueChange={field.onChange}
														defaultValue={field.value}
													>
														<FormControl>
															<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																<SelectValue
																	placeholder=""
																	className="text-xs text-neutral-10"
																/>
															</SelectTrigger>
														</FormControl>
														<SelectContent className="shadow-none">
															{pubSubTypes.map(type => (
																<SelectItem
																	className="cursor-pointer text-xs py-3 hover:bg-transparent"
																	value={type.uid}
																	key={type.uid}
																>
																	<span className="text-neutral-10">
																		{type.name}
																	</span>
																</SelectItem>
															))}
														</SelectContent>
													</Select>
												</FormItem>
											)}
										/>
									</div>
									<div>
										<FormField
											control={createOutgoingSourceForm.control}
											name="workers"
											render={({ field, fieldState }) => (
												<FormItem className="w-full block">
													<div className="w-full mb-2 flex items-center justify-between">
														<FormLabel className="flex items-center gap-2">
															<span className="text-xs/5 text-neutral-9 ">
																Workers
															</span>
															<TooltipProvider>
																<Tooltip>
																	<TooltipTrigger
																		asChild
																		className="cursor-pointer"
																	>
																		<span className="text-xs scale-90">ⓘ</span>
																	</TooltipTrigger>
																	<TooltipContent className="text-xs/5 text-neutral-9 bg-white-100 w-[300px] border border-neutral-4">
																		This specifies the number of consumers you
																		want polling messages from your queue. For
																		Kafka sources, the number of partitions for
																		the topic should match the number of workers
																	</TooltipContent>
																</Tooltip>
															</TooltipProvider>
														</FormLabel>
													</div>
													<FormControl>
														<Input
															type="number"
															step={1}
															pattern="\d*"
															min={0}
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
															placeholder="Workers"
															{...field}
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>
									</div>

									{createOutgoingSourceForm.watch('type') == 'google' && (
										<section>
											<h3 className="text-xs font-semibold">
												Configure Google Pub/Sub
											</h3>
										</section>
									)}
								</div>
							</div>
							{/* Submit Button */}
							<div className="flex justify-end mt-6 w-full">
								<Button
									type="submit"
									disabled={
										isCreating || !licenses.includes('WEBHOOK_TRANSFORMATIONS')
									}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isCreating ? 'Creating...' : 'Create'} Source
									<ChevronRight className="stroke-white-100" />
								</Button>
							</div>
						</form>
					</Form>
				)}
			</section>

			<Dialog open={hasCreatedIncomingSource} onOpenChange={(isOpen) => {
				if(!isOpen){
					return navigate({
						to: "/projects/$projectId/sources",
						params: {projectId}
					})
				}
			}}>
				<DialogContent
					className="max-w-[480px] rounded-lg p-4"
					aria-describedby={undefined}
				>
					<DialogHeader>
						<DialogTitle className="text-base font-semibold text-start mb-4">
							Source URL
						</DialogTitle>
						<div>
							<p className="text-xs/5 text-neutral-10 mb-4 text-start">
								Copy the source URL below into your source platform to start
								receiving webhook events.
							</p>

							<div className="flex items-center justify-between w-full h-[50px] border border-neutral-a3 pr-2 pl-3 rounded-md">
								<span className="text-xs text-neutral-11 font-normal">
									{sourceUrl}
								</span>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									className="asbolute right-[1%] top-0 h-full py-2 hover:bg-transparent pr-1 pl-0"
									onClick={() => {
										window.navigator.clipboard.writeText(sourceUrl).then();
										// TODO show toast message on copy successful
									}}
								>
									<CopyIcon className="opacity-50" aria-hidden="true" />
									<span className="sr-only">copy source url</span>
								</Button>
							</div>
						</div>
					</DialogHeader>
					<DialogFooter>
						<DialogClose asChild>
							<div className="flex justify-end">
								{' '}
								<Button
									onClick={() => setHasCreatedIncomingSource(false)}
									type="button"
									variant="ghost"
									className="w-fit hover:bg-new.primary-400 text-white-100 hover:text-white-100 bg-new.primary-400 px-3 py-4 text-xs"
								>
									Close
								</Button>
							</div>
						</DialogClose>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
