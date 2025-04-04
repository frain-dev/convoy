import { z } from 'zod';
import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { DiffEditor, Editor } from '@monaco-editor/react';
import { useForm, type RegisterOptions } from 'react-hook-form';
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';

import { ChevronRight, Info, CopyIcon, SaveIcon } from 'lucide-react';

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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
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
	DialogDescription,
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

import { useLicenseStore } from '@/store';
import * as authService from '@/services/auth.service';
import * as sourcesService from '@/services/sources.service';
import * as projectsService from '@/services/projects.service';

import uploadIcon from '../../../../../assets/img/upload.png';
import githubIcon from '../../../../../assets/img/github.png';
import shopifyIcon from '../../../../../assets/img/shopify.png';
import twitterIcon from '../../../../../assets/img/twitter.png';
import docIcon from '../../../../../assets/img/doc-icon-primary.svg';
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

import type { DragEvent, ChangeEvent } from 'react';

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

export const Route = createFileRoute('/projects_/$projectId/sources/$sourceId')(
	{
		component: RouteComponent,
		async loader({ params }) {
			const source = await sourcesService.getSourceDetails(params.sourceId);
			const perms = await authService.getUserPermissions();
			const project = await projectsService.getProject(params.projectId);

			return {
				source,
				project,
				canManageSources: perms.includes('Sources|MANAGE'),
			};
		},
	},
);

type SourceType = (typeof sourceVerifications)[number]['uid'];

const sourceVerifications = [
	{ uid: 'noop', name: 'None' },
	{ uid: 'hmac', name: 'HMAC' },
	{ uid: 'basic_auth', name: 'Basic Auth' },
	{ uid: 'api_key', name: 'API Key' },
	{ uid: 'github', name: 'Github' },
	{ uid: 'shopify', name: 'Shopify' },
	{ uid: 'twitter', name: 'Twitter' },
] as const;

const pubSubTypes = [
	{ uid: 'google', name: 'Google Pub/Sub' },
	{ uid: 'kafka', name: 'Kafka' },
	{ uid: 'sqs', name: 'AWS SQS' },
	{ uid: 'amqp', name: 'AMQP / RabbitMQ' },
] as const;

const AWSRegions = [
	{ uid: 'us-east-2', name: 'US East (Ohio)' },
	{ uid: 'us-east-1', name: 'US East (N. Virginia)' },
	{ uid: 'us-west-1', name: 'US West (N. California)' },
	{ uid: 'us-west-2', name: 'US West (Oregon)' },
	{ uid: 'af-south-1', name: 'Africa (Cape Town)' },
	{ uid: 'ap-east-1', name: 'Asia Pacific (Hong Kong)' },
	{ uid: 'ap-south-2', name: 'Asia Pacific (Hyderabad)' },
	{ uid: 'ap-southeast-3', name: 'Asia Pacific (Jakarta)' },
	{ uid: 'ap-southeast-4', name: 'Asia Pacific (Melbourne)' },
	{ uid: 'ap-south-1', name: 'Asia Pacific (Mumbai)' },
	{ uid: 'ap-northeast-3', name: 'Asia Pacific (Osaka)' },
	{ uid: 'ap-northeast-2', name: 'Asia Pacific (Seoul)' },
	{ uid: 'ap-southeast-1', name: 'Asia Pacific (Singapore)' },
	{ uid: 'ap-southeast-2', name: 'Asia Pacific (Sydney)' },
	{ uid: 'ap-northeast-1', name: 'Asia Pacific (Tokyo)' },
	{ uid: 'ca-central-1', name: 'Canada (Central)' },
	{ uid: 'eu-central-1', name: 'Europe (Frankfurt)' },
	{ uid: 'eu-west-1', name: 'Europe (Ireland)' },
	{ uid: 'eu-west-2', name: 'Europe (London)' },
	{ uid: 'eu-south-1', name: 'Europe (Milan)' },
	{ uid: 'eu-west-3', name: 'Europe (Paris)' },
	{ uid: 'eu-south-2', name: 'Europe (Spain)' },
	{ uid: 'eu-north-1', name: 'Europe (Stockholm)' },
	{ uid: 'eu-central-2', name: 'Europe (Zurich)' },
	{ uid: 'me-south-1', name: 'Middle East (Bahrain)' },
	{ uid: 'me-central-1', name: 'Middle East (UAE)' },
	{ uid: 'sa-east-1', name: 'South America (São Paulo)' },
	{ uid: 'us-gov-east-1', name: 'AWS GovCloud (US-East)' },
	{ uid: 'us-gov-west-1', name: 'AWS GovCloud (US-West)' },
] as const;

const defaultBody = {
	id: 'Sample-1',
	name: 'Sample 1',
	description: 'This is sample data #1',
};

const defaultOutput = { previous: '', current: '' };

const IncomingSourceFormSchema = z
	.object({
		name: z.string().min(1, 'Enter new source name'),
		type: z.enum([
			sourceVerifications[0].uid,
			...sourceVerifications.slice(1).map(t => t.uid),
		]),
		is_disabled: z.boolean().optional(),
		config: z.object({
			encoding: z.enum(['base64', 'hex', '']).optional(),
			hash: z.enum(['SHA256', 'SHA512', '']).optional(),
			header: z.string().optional(),
			secret: z.string().optional(),
			username: z.string().optional(),
			password: z.string().optional(),
			header_name: z.string().optional(),
			header_value: z.string().optional(),
		}),
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

const OutgoingSourceFormSchema = z
	.object({
		name: z
			.string({ required_error: 'Enter new source name' })
			.min(1, 'Enter new source name'),
		type: z.literal('pub_sub'),
		is_disabled: z.boolean(),
		pub_sub: z.object({
			type: z
				.enum(['', ...pubSubTypes.map(t => t.uid)])
				.refine(v => (v == '' ? false : true), {
					message: 'Select source type',
					path: ['type'],
				}),
			workers: z
				.string({ required_error: 'Enter number of workers' })
				.min(1, 'Enter number of workers')
				.refine(v => v !== null)
				.transform(Number),
			google: z
				.object({
					project_id: z.string(),
					service_account: z.string(),
					subscription_id: z.string(),
				})
				.optional(),
			kafka: z
				.object({
					brokers: z.array(z.string()).optional(),
					consumer_group_id: z.string().optional(),
					topic_name: z.string(),
					auth: z
						.object({
							type: z.enum(['plain', 'scram', '']),
							tls: z.enum(['enabled', 'disabled', '']),
							username: z.string().optional(),
							password: z.string().optional(),
							hash: z.enum(['SHA256', 'SHA512', '']).optional(),
						})
						.optional(),
				})
				.optional(),
			sqs: z
				.object({
					queue_name: z.string(),
					access_key_id: z.string(),
					secret_key: z.string(),
					default_region: z
						.enum(['', ...AWSRegions.map(reg => reg.uid)])
						.optional(),
				})
				.optional(),
			amqp: z
				.object({
					schema: z.string(),
					host: z.string(),
					port: z.string().transform(Number),
					queue: z.string(),
					deadLetterExchange: z.string().optional(),
					vhost: z.string().optional(),
					auth: z
						.object({
							user: z.string(),
							password: z.string(),
						})
						.optional(),
					bindExchange: z
						.object({
							exchange: z.string(),
							routingKey: z.string(),
						})
						.optional(),
				})
				.optional(),
		}),
		showKafkaAuth: z.boolean().optional(),
		showAMQPAuth: z.boolean().optional(),
		showAMQPBindExhange: z.boolean().optional(),
		showTransform: z.boolean().optional(),
	})
	.refine(
		({ pub_sub }) => {
			if (pub_sub.type !== 'google') return true;

			if (
				!pub_sub.google?.project_id ||
				!pub_sub.google?.subscription_id ||
				!pub_sub.google?.service_account
			) {
				return false;
			}

			return true;
		},
		({ pub_sub }) => {
			if (!pub_sub.google?.project_id) {
				return {
					message: 'Project ID is required',
					path: ['pub_sub.google.project_id'],
				};
			}

			if (!pub_sub.google?.subscription_id) {
				return {
					message: 'Subscription ID is required',
					path: ['pub_sub.google.subscription_id'],
				};
			}
			if (!pub_sub.google?.service_account) {
				return {
					message: 'Service account is required',
					path: ['pub_sub.google.service_account'],
				};
			}

			return { message: '', path: [] };
		},
	)
	.refine(
		({ showKafkaAuth, pub_sub }) => {
			if (pub_sub.type !== 'kafka') return true;
			if (!pub_sub.kafka?.brokers?.length) return false;
			if (!pub_sub.kafka.topic_name) return false;
			if (!showKafkaAuth) return true;

			let hasInvalidValue =
				!pub_sub.kafka?.auth?.type || !pub_sub.kafka?.auth?.tls;
			if (hasInvalidValue) return false;

			hasInvalidValue =
				!pub_sub.kafka?.auth?.username || !pub_sub.kafka?.auth?.password;
			if (hasInvalidValue) return false;

			hasInvalidValue =
				pub_sub.kafka?.auth?.type == 'scram' && !pub_sub.kafka?.auth?.hash;
			if (hasInvalidValue) return false;

			return true;
		},
		({ pub_sub }) => {
			if (!pub_sub.kafka?.topic_name) {
				return {
					message: 'Topic name is required',
					path: ['pub_sub.kafka.topic_name'],
				};
			}

			if (!pub_sub.kafka?.brokers?.length) {
				return {
					message: 'A minimum of one broker address is required',
					path: ['pub_sub.kafka.brokers'],
				};
			}

			if (!pub_sub.kafka?.auth?.type) {
				return {
					message: 'Please select authentication type',
					path: ['pub_sub.kafka.auth.type'],
				};
			}

			if (!pub_sub.kafka?.auth?.tls) {
				return {
					message: 'Enable or disable TLS',
					path: ['pub_sub.kafka.auth.tls'],
				};
			}

			if (!pub_sub.kafka?.auth?.hash) {
				return {
					message: 'has is required when authentication type is scram',
					path: ['pub_sub.kafka.auth.hash'],
				};
			}

			if (!pub_sub.kafka?.auth?.username) {
				return {
					message: 'Username is required',
					path: ['pub_sub.kafka.auth.username'],
				};
			}

			if (!pub_sub.kafka?.auth?.password) {
				return {
					message: 'Password is required',
					path: ['pub_sub.kafka.auth.password'],
				};
			}

			return { message: '', path: [''] };
		},
	)
	.refine(
		({ pub_sub }) => {
			if (pub_sub.type !== 'sqs') return true;

			if (
				!pub_sub.sqs?.default_region ||
				!pub_sub.sqs?.queue_name ||
				!pub_sub.sqs?.access_key_id ||
				!pub_sub.sqs?.secret_key
			)
				return false;
			return true;
		},
		{
			message: 'Select AWS default region',
			path: ['pub_sub.sqs.default_region'],
		},
	)
	.refine(
		({ pub_sub, showAMQPAuth, showAMQPBindExhange }) => {
			if (pub_sub.type != 'amqp') return true;

			if (
				!pub_sub.amqp?.schema ||
				!pub_sub.amqp?.host ||
				!pub_sub.amqp?.port ||
				!pub_sub.amqp?.queue
			) {
				return false;
			}

			if (!showAMQPAuth) return true;

			if (!pub_sub.amqp?.auth?.user || !pub_sub.amqp?.auth?.password) {
				return false;
			}

			if (!showAMQPBindExhange) return true;

			if (
				!pub_sub.amqp?.bindExchange?.exchange ||
				!pub_sub.amqp?.bindExchange?.routingKey
			) {
				return false;
			}

			return true;
		},
		({ pub_sub, showAMQPAuth, showAMQPBindExhange }) => {
			if (!pub_sub.amqp?.schema)
				return {
					message: 'Schema is required',
					path: ['pub_sub.amqp.schema'],
				};

			if (!pub_sub.amqp?.host)
				return {
					message: 'Host is required',
					path: ['pub_sub.amqp.host'],
				};

			if (!pub_sub.amqp?.port)
				return {
					message: 'Port is required',
					path: ['pub_sub.amqp.port'],
				};

			if (!pub_sub.amqp?.queue)
				return {
					message: 'Queue is required',
					path: ['pub_sub.amqp.queue'],
				};

			if (showAMQPAuth && !pub_sub.amqp?.auth?.user)
				return {
					message: 'User is required when authentication is enabled',
					path: ['pub_sub.amqp.auth.user'],
				};

			if (showAMQPAuth && !pub_sub.amqp?.auth?.password)
				return {
					message: 'Password is required when authentication is enabled',
					path: ['pub_sub.amqp.auth.password'],
				};

			if (showAMQPBindExhange && !pub_sub.amqp?.bindExchange?.exchange)
				return {
					message: 'Exchange is required when binding exchange is enabled',
					path: ['pub_sub.amqp.bindExchange.exchange'],
				};

			if (showAMQPBindExhange && !pub_sub.amqp?.bindExchange?.routingKey)
				return {
					message: 'Routing key is required when binding exchange is enabled',
					path: ['pub_sub.amqp.bindExchange.routingKey'],
				};

			return { message: '', path: [] };
		},
	);

const defaultTranformFnBody = `/*  1. While you can write multiple functions, the main
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
      }`;

const defaultTransformFnHeader = `/* 1. While you can write multiple functions, the main function
      called for your transformation is the transform function.
      
      2. The only argument acceptable in the transform function is
      the payload data.
      
      3. The transform method must return a value.
      
      4. Console logs lust be written like this
      console.log('%j', logged_item) to get printed in the log below. */
      
      function transform(payload) {
      // Transform function here
      return payload;
      }`;

function RouteComponent() {
	const navigate = useNavigate();
	const { licenses } = useLicenseStore();
	const { source, project, canManageSources } = Route.useLoaderData();
	const [sourceUrl, setSourceUrl] = useState('');
	const [isUpdating, setIsUpdating] = useState(false);
	const [hasCreatedIncomingSource, setHasCreatedIncomingSource] =
		useState(false);
	const [selectedFile, setSelectedFile] = useState<File | null>(null);
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
		source.body_function || defaultTranformFnBody,
	);
	const [transformFnHeader, setTransformFnHeader] = useState<string>(
		source.header_function || defaultTransformFnHeader,
	);
	const [bodyOutput, setBodyOutput] = useState<FuncOutput>(defaultOutput);
	const [headerOutput, setHeaderOutput] = useState<FuncOutput>(defaultOutput);
	const [bodyLogs, setBodyLogs] = useState<string[]>([]);
	const [headerLogs, setHeaderLogs] = useState<string[]>([]);
	const [transformFn, setTransformFn] = useState<string>();
	const [headerTransformFn, setHeaderTransformFn] = useState<string>();
	const [hasSavedFn, setHasSavedFn] = useState(false);

	const incomingForm = useForm<z.infer<typeof IncomingSourceFormSchema>>({
		resolver: zodResolver(IncomingSourceFormSchema),
		defaultValues: {
			name: source.name,
			type: source.provider || source.verifier.type || 'noop',
			is_disabled: true,
			config: {
				hash: source.verifier.hmac.hash,
				encoding: source.verifier.hmac.encoding,
				header: source.verifier.hmac.header,
				secret: source.verifier.hmac.secret,
				username: source.verifier.basic_auth.username,
				password: source.verifier.basic_auth.password,
				header_name: source.verifier.api_key.header_name,
				header_value: source.verifier.api_key.header_value,
			},
			custom_response: {
				content_type: source.custom_response.content_type,
				body: source.custom_response.body,
			},
			idempotency_keys: source.idempotency_keys,
			showHmac: source.verifier.type == 'hmac',
			showBasicAuth: source.verifier.type == 'basic_auth',
			showAPIKey: source.verifier.type == 'api_key',
			showGithub: source.provider == 'github',
			showTwitter: source.provider == 'twitter',
			showShopify: source.provider == 'shopify',
			showCustomResponse: source.custom_response.content_type ? true : false,
			showIdempotency: source.idempotency_keys?.length ? true : false,
		},
		mode: 'onTouched',
	});

	const outgoingForm = useForm<z.infer<typeof OutgoingSourceFormSchema>>({
		resolver: zodResolver(OutgoingSourceFormSchema),
		defaultValues: {
			name: source.name,
			type: 'pub_sub',
			is_disabled: true,
			body_function: JSON.stringify(source.body_function),
			header_function: JSON.stringify(source.header_function),
			pub_sub: {
				type: source.pub_sub.type,
				workers: source.pub_sub.workers,
				google: source.pub_sub.google,
				kafka: {
					brokers: source.pub_sub.kafka?.brokers
						? source.pub_sub.kafka.brokers
						: [],
					consumer_group_id: source.pub_sub.kafka?.consumer_group_id,
					topic_name: source.pub_sub.kafka?.topic_name,
					auth: {
						type: source.pub_sub.kafka?.auth?.type || '',
						// TODO find out what the real response from this is
						tls: source.pub_sub.kafka?.auth?.tls ? 'enabled' : 'disabled',
						username: source.pub_sub.kafka?.auth?.username || '',
						password: source.pub_sub.kafka?.auth?.password || '',
						hash: source.pub_sub.kafka?.auth?.hash || '',
					},
				},
				sqs: {
					queue_name: source.pub_sub.sqs?.queue_name,
					access_key_id: source.pub_sub.sqs?.access_key_id,
					secret_key: source.pub_sub.sqs?.secret_key,
					// @ts-expect-error this balances out in reality
					default_region: source.pub_sub.sqs?.default_region,
				},
				amqp: {
					schema: source.pub_sub.amqp?.schema,
					host: source.pub_sub.amqp?.host,
					// @ts-expect-error this balances out in reality
					port: source.pub_sub.amqp?.port,
					queue: source.pub_sub.amqp?.queue,
					deadLetterExchange: source.pub_sub.amqp?.deadLetterExchange,
					vhost: source.pub_sub.amqp?.vhost,
					auth: {
						password: source.pub_sub.amqp?.auth?.password || '',
						user: source.pub_sub.amqp?.auth?.user || '',
					},
					bindedExchange: {
						exchange: source.pub_sub.amqp?.bindedExchange?.exchange || '',
						routingKey: source.pub_sub.amqp?.bindedExchange?.routingKey || '""',
					},
				},
			},
			showKafkaAuth: !!source.pub_sub.kafka?.auth.password,
			showAMQPAuth: !!source.pub_sub.amqp?.auth.password,
			showAMQPBindExhange: !!source.pub_sub.amqp?.bindedExchange.exchange,
			showTransform: false,
		},
		mode: 'onTouched',
	});

	function getVerifierType(
		type: SourceType,
		config: z.infer<typeof IncomingSourceFormSchema>['config'],
	) {
		const obj: Record<string, string> = {};

		if (type == 'hmac') {
			return {
				type: 'hmac',
				hmac: Object.entries(config).reduce((acc, record: [string, string]) => {
					const [key, val] = record;
					if (['encoding', 'hash', 'header', 'secret'].includes(key)) {
						acc[key] = val;
						return acc;
					}
					return acc;
				}, obj),
			};
		}

		if (type == 'basic_auth') {
			return {
				type: 'basic_auth',
				basic_auth: Object.entries(config).reduce(
					(acc, record: [string, string]) => {
						const [key, val] = record;
						if (['password', 'username'].includes(key)) {
							acc[key] = val;
							return acc;
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
		v: z.infer<typeof IncomingSourceFormSchema>,
	) {
		return {
			name: v.name,
			is_disabled: v.is_disabled,
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

	async function createIncomingSource(
		raw: z.infer<typeof IncomingSourceFormSchema>,
	) {
		const payload = transformIncomingSource(raw);
		setIsUpdating(true);
		try {
			const response = await sourcesService.createSource(payload);
			setSourceUrl(response.url);
			setHasCreatedIncomingSource(true);
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdating(false);
		}
	}

	function onFileInputChange(
		e: ChangeEvent<HTMLInputElement>,
		field: RegisterOptions,
	) {
		if (e.target?.files?.length) {
			const file = e.target.files[0];
			// ensure 5kb limit
			if (file.size > 5 * 1024) {
				setSelectedFile(null);
				field.onChange?.('');
				// TODO: Show error toast/message
				console.error('File size exceeds 5kb limit');
				return;
			}
			setSelectedFile(file);
			// Handle the file here
			const reader = new FileReader();
			reader.onload = event => {
				try {
					JSON.parse(event.target?.result as string);
					// Parse JSON to the form to check if it's valid
					if (reader.result) {
						field.onChange?.(btoa(reader.result.toString()));
					}
				} catch (error) {
					console.error('Invalid JSON file');
				}
			};
			reader.readAsText(file, 'UTF-8');
		}
	}

	function onFileInputDrop(
		e: DragEvent<HTMLInputElement>,
		field: RegisterOptions,
	) {
		e.preventDefault();
		e.stopPropagation();
		if (e.dataTransfer.files && e.dataTransfer.files[0]) {
			const file = e.dataTransfer.files[0];
			if (file.size > 5 * 1024) {
				// TODO: Show error toast/message
				setSelectedFile(null);
				console.error('File size exceeds 5kb limit');
				return;
			}
			setSelectedFile(file);
			const reader = new FileReader();
			reader.onload = event => {
				try {
					// Parse JSON to the form to check if it's valid
					JSON.parse(event.target?.result as string);
					if (reader.result) {
						field.onChange?.(btoa(reader.result.toString()));
					}
				} catch (error) {
					console.error('Invalid JSON file');
				}
			};
			reader.readAsText(file);
		}
	}

	function transformOutgoingSource(
		raw: z.infer<typeof OutgoingSourceFormSchema>,
	) {
		const payload = {
			name: raw.name,
			type: raw.type,
			is_disabled: true,
			pub_sub: {
				workers: raw.pub_sub.workers,
				type: raw.pub_sub.type,
			},
			body_function: raw.showTransform && transformFn ? transformFn : null,
			header_function:
				raw.showTransform && headerTransformFn ? headerTransformFn : null,
		};

		if (raw.pub_sub.type == 'google') {
			return {
				...payload,
				pub_sub: {
					...payload.pub_sub,
					google: raw.pub_sub.google,
				},
			};
		}

		if (raw.pub_sub.type == 'kafka') {
			const kafka = raw.pub_sub.kafka;
			return {
				...payload,
				pub_sub: {
					...payload.pub_sub,
					kafka: {
						...kafka,
						auth: {
							...kafka?.auth,
							tls: kafka?.auth?.tls == 'enabled' ? true : false,
						},
					},
				},
			};
		}

		if (raw.pub_sub.type == 'sqs') {
			const sqs = raw.pub_sub.sqs;
			return {
				...payload,
				pub_sub: {
					...payload.pub_sub,
					sqs,
				},
			};
		}

		if (raw.pub_sub.type == 'amqp') {
			const amqp = raw.pub_sub.amqp;
			return {
				...payload,
				pub_sub: {
					...payload.pub_sub,
					amqp,
				},
			};
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
					response.log.toReversed() || [
						'Transform function executed successfully',
					],
				);
			} else {
				setHeaderOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));
				setHeaderLogs(
					response.log.toReversed() || [
						'Transform function executed successfully',
					],
				);
			}

			setIsTransformPassed(true);
			setIsTestingFunction(false);
			setShowConsole(bodyLogs.length || headerLogs.length ? true : false);

			if (type === 'body') {
				setTransformFn(transformFunc);
			} else {
				setHeaderTransformFn(transformFunc);
			}
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

	async function createOutgoingSource(
		raw: z.infer<typeof OutgoingSourceFormSchema>,
	) {
		const payload = transformOutgoingSource(raw);
		console.log(payload);
		setIsUpdating(true);
		try {
			/* const res =  */ await sourcesService.createSource(payload);
			// TODO notify UI
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdating(false);
		}
	}

	return (
		<DashboardLayout showSidebar={false}>
			<section className="flex flex-col p-2 max-w-[770px] min-w-[600px] w-full m-auto my-4 gap-y-6">
				<div className="flex justify-start items-center gap-2">
					<Link
						to="/projects/$projectId/sources"
						params={{ projectId: project.uid }}
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
					<Form {...incomingForm}>
						<form
							onSubmit={incomingForm.handleSubmit(createIncomingSource)}
							className="w-full"
						>
							<div className="border border-neutral-4 rounded-8px p-6 w-full">
								<div className="grid grid-cols-1 w-full gap-y-5">
									<h3 className="font-semibold text-xs text-neutral-10">
										Pre-configured Sources
									</h3>
									<div className="flex flex-col gap-y-2">
										<ToggleGroup
											type="single"
											className="flex justify-start items-center gap-x-4"
											value={incomingForm.watch('type')}
											onValueChange={(v: SourceType) => {
												incomingForm.setValue('type', v);
												incomingForm.setValue(
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
													incomingForm.watch('type') === 'github' &&
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
													incomingForm.watch('type') === 'shopify' &&
														'border-new.primary-400',
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
													incomingForm.watch('type') === 'twitter' &&
														'border-new.primary-400',
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
											control={incomingForm.control}
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
											control={incomingForm.control}
											render={({ field }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Source Verification
													</FormLabel>
													<Select
														value={incomingForm.watch('type')}
														onValueChange={(v: SourceType) => {
															field.onChange(v);
															if (
																['github', 'shopify', 'twitter'].includes(v)
															) {
																incomingForm.setValue(
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
									{incomingForm.watch('type') == 'hmac' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<h4 className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure HMAC
											</h4>

											<FormField
												name="config.encoding"
												control={incomingForm.control}
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
												control={incomingForm.control}
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
												control={incomingForm.control}
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
												control={incomingForm.control}
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
									{incomingForm.watch('type') == 'basic_auth' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure Basic Auth
											</p>

											<FormField
												name="config.username"
												control={incomingForm.control}
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
												control={incomingForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Password
														</FormLabel>
														<FormControl>
															<Input
																type="text"
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
									{incomingForm.watch('type') == 'api_key' && (
										<div className="grid grid-cols-2 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												Configure API Key
											</p>

											<FormField
												name="config.header_name"
												control={incomingForm.control}
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
												control={incomingForm.control}
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
										incomingForm.watch('type'),
									) && (
										<div className="grid grid-cols-1 gap-x-5 gap-y-4">
											<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
												{incomingForm.watch('type')} Credentials
											</p>

											<FormField
												name="config.secret"
												control={incomingForm.control}
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
											control={incomingForm.control}
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
											control={incomingForm.control}
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
									{incomingForm.watch('showCustomResponse') && (
										<div className="border-l border-new.primary-25 pl-4 flex flex-col gap-y-4">
											<h3 className="text-xs text-neutral-10 font-semibold">
												Custom Response
											</h3>

											<FormField
												name="custom_response.content_type"
												control={incomingForm.control}
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
												control={incomingForm.control}
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
									{incomingForm.watch('showIdempotency') && (
										<div className="border-l border-new.primary-25 pl-4 flex flex-col gap-y-4">
											<h3 className="text-xs text-neutral-10 font-semibold">
												Idempotency Config
											</h3>

											<FormField
												name="idempotency_keys"
												control={incomingForm.control}
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel className="flex items-center gap-x-1">
															<span className="text-neutral-9 text-xs">
																Key locations
															</span>
															<TooltipProvider>
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
															</TooltipProvider>
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
										!incomingForm.formState.isValid ||
										isUpdating ||
										!canManageSources
									}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isUpdating ? 'Updating...' : 'Update'} Source
									<ChevronRight className="stroke-white-100" />
								</Button>
							</div>
						</form>
					</Form>
				)}

				{project?.type == 'outgoing' && (
					<Form {...outgoingForm}>
						<form
							onSubmit={outgoingForm.handleSubmit(createOutgoingSource)}
							className="w-full"
						>
							<div className="border border-neutral-4 rounded-8px p-6 w-full">
								<div className="grid grid-cols-1 w-full gap-y-5">
									<div>
										<FormField
											control={outgoingForm.control}
											name="name"
											render={({ field, fieldState }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-xs/5 text-neutral-9">
														Source name
													</FormLabel>
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
											name="pub_sub.type"
											control={outgoingForm.control}
											render={({ field }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Source Type
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
														<FormMessageWithErrorIcon />
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
											control={outgoingForm.control}
											name="pub_sub.workers"
											render={({ field, fieldState }) => (
												<FormItem className="">
													<div className="space-y-2">
														<FormLabel className="flex items-center gap-2 mb-2">
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

									{outgoingForm.watch('pub_sub.type') == 'google' && (
										<section className="grid grid-cols-2 gap-x-5 gap-y-4">
											<h3 className="text-xs font-semibold col-span-full">
												Configure Google Pub/Sub
											</h3>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.google.project_id"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative block">
															<div className="w-full mb-2">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Project ID
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.google.subscription_id"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative block">
															<div className="w-full mb-2">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Subscription ID
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-full">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.google.service_account"
													render={({ field }) => (
														<FormItem className="w-full block">
															<div className="w-full mb-2">
																<FormLabel>
																	<span className="text-xs/5 text-neutral-9">
																		Service Account
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
																				Service accounts provide a way to manage
																				authentication into your Google Pub/Sub.
																			</p>
																		</TooltipContent>
																	</Tooltip>
																</FormLabel>
															</div>
															<FormControl>
																<div className="border-dashed border-2 border-neutral-4 rounded-md hover:border-new.primary-100 transition-all cursor-pointer">
																	<div className="relative h-[100px]">
																		<div
																			className="absolute"
																			style={{ top: '15%', left: '30%' }}
																		>
																			<div className="flex flex-col items-center justify-center gap-2">
																				<img
																					src={uploadIcon}
																					alt="upload"
																					className="w-8 h-8"
																				/>
																				<p className="text-center text-xs text-neutral-11 mx-auto">
																					<span className="text-primary-100 font-semibold">
																						Click to upload
																					</span>{' '}
																					or drag and drop JSON (max 5kb)
																				</p>
																				{field.value && (
																					<div className="flex items-center mt-2">
																						<span className="text-xs text-neutral-10 w-full">
																							File selected
																							{selectedFile
																								? `: ${selectedFile.name} (${(selectedFile.size / 1000).toFixed(2)}kB)`
																								: ''}
																						</span>
																						<Button
																							type="button"
																							variant="ghost"
																							size="sm"
																							className="ml-2 p-5 h-auto hover:bg-transparent"
																							onClick={() => {
																								field.onChange('');
																								const fileInput =
																									document.getElementById(
																										'service_account_file',
																									) as HTMLInputElement;
																								fileInput.value = '';
																								setSelectedFile(null);
																							}}
																						>
																							<svg
																								width="14"
																								height="14"
																								className="fill-transparent stroke-destructive"
																							>
																								<use xlinkHref="#delete-icon2"></use>
																							</svg>
																						</Button>
																					</div>
																				)}
																			</div>
																		</div>
																		<input
																			name="pub_sub.google.service_account"
																			type="file"
																			id="service_account_file"
																			className="opacity-0 w-full h-[80px] cursor-pointer"
																			accept=".json"
																			onChange={e =>
																				onFileInputChange(e, field)
																			}
																			onDragOver={e => {
																				e.preventDefault();
																				e.stopPropagation();
																			}}
																			onDrop={e => onFileInputDrop(e, field)}
																		/>
																	</div>
																</div>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</section>
									)}

									{outgoingForm.watch('pub_sub.type') == 'kafka' && (
										<section className="grid grid-cols-2 gap-x-5 gap-y-4">
											<h3 className="text-xs font-semibold col-span-full flex items-center justify-start gap-x-4">
												<span>Configure Kafka</span>{' '}
												<a
													href="https://docs.getconvoy.io/product-manual/sources#kafka"
													className="flex justify-start items-center gap-x-2"
												>
													<img
														src={docIcon}
														alt="convoy kafka docs"
														className="h-4 w-4"
													/>
													<span className="font-medium text-xs text-new.primary-400 whitespace-normal">
														Docs
													</span>
												</a>
											</h3>
											<div className="col-span-full">
												<FormField
													name="pub_sub.kafka.brokers"
													control={outgoingForm.control}
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-10">
																Broker Addresses
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
																Set the value of each input with a coma (,)
															</p>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													name="pub_sub.kafka.topic_name"
													control={outgoingForm.control}
													render={({ field, fieldState }) => (
														<FormItem className="w-full space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Topic Name
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													name="pub_sub.kafka.consumer_group_id"
													control={outgoingForm.control}
													render={({ field, fieldState }) => (
														<FormItem className="w-full space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Consumer ID
															</FormLabel>
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
													name="showKafkaAuth"
													control={outgoingForm.control}
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label={() => (
																		<span className="font-semibold text-xs">
																			Authentication
																		</span>
																	)}
																	// @ts-expect-error the default value is boolean
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>
											</div>

											<div className="space-y-2 col-span-full pl-4 border-l border-l-new.primary-25">
												{/* Authentication Type */}
												{outgoingForm.watch('showKafkaAuth') && (
													<div className="col-span-full">
														<FormField
															name="pub_sub.kafka.auth.type"
															control={outgoingForm.control}
															render={({ field }) => (
																<FormItem className="w-full relative mb-6 block">
																	<p className="text-xs/5 text-neutral-9 mb-2">
																		Authentiation Type
																	</p>
																	<div className="flex w-full gap-x-6">
																		{[
																			{ label: 'plain', value: 'plain' },
																			{ label: 'scram', value: 'scram' },
																		].map(({ label, value }) => {
																			return (
																				<FormControl
																					className="w-full"
																					key={label}
																				>
																					<label
																						className={cn(
																							'cursor-pointer border border-primary-100 flex items-start gap-x-2 p-4 rounded-sm',
																							field.value === value
																								? 'border-new.primary-300 bg-[#FAFAFE]'
																								: 'border-neutral-5',
																						)}
																						htmlFor={`kafka_auth_type_${label}`}
																					>
																						<span className="sr-only">
																							{label}
																						</span>
																						<Input
																							type="radio"
																							id={`kafka_auth_type_${label}`}
																							{...field}
																							value={value}
																							className="shadow-none h-4 w-fit"
																						/>
																						<div className="flex flex-col gap-y-1">
																							<p className="w-full text-xs text-neutral-10 font-semibold">
																								{label}
																							</p>
																						</div>
																					</label>
																				</FormControl>
																			);
																		})}
																		<FormMessageWithErrorIcon />
																	</div>
																</FormItem>
															)}
														/>
													</div>
												)}

												{/* TLS */}
												{outgoingForm.watch('showKafkaAuth') && (
													<div className="col-span-full">
														<FormField
															control={outgoingForm.control}
															name="pub_sub.kafka.auth.tls"
															render={({ field }) => (
																<FormItem className="w-full relative mb-6 block">
																	<p className="text-xs/5 text-neutral-9 mb-2">
																		TLS
																	</p>
																	<div className="flex w-full gap-x-6">
																		{[
																			{ label: 'Enabled', value: 'enabled' },
																			{ label: 'Disabled', value: 'disabled' },
																		].map(({ label, value }) => {
																			return (
																				<FormControl
																					className="w-full"
																					key={label}
																				>
																					<label
																						className={cn(
																							'cursor-pointer border border-primary-100 flex items-start gap-x-2 p-4 rounded-sm',
																							field.value === value
																								? 'border-new.primary-300 bg-[#FAFAFE]'
																								: 'border-neutral-5',
																						)}
																						htmlFor={`kafka_tls_${label}`}
																					>
																						<span className="sr-only">
																							{label}
																						</span>
																						<Input
																							type="radio"
																							id={`kafka_tls_${label}`}
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

												{/* Username/Password */}
												{outgoingForm.watch('showKafkaAuth') && (
													<div className="col-span-full grid grid-cols-2 gap-x-5">
														<div>
															<FormField
																name="pub_sub.kafka.auth.username"
																control={outgoingForm.control}
																render={({ field, fieldState }) => (
																	<FormItem className="w-full space-y-2">
																		<FormLabel className="text-xs/5 text-neutral-9">
																			Username
																		</FormLabel>
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
																name="pub_sub.kafka.auth.password"
																control={outgoingForm.control}
																render={({ field, fieldState }) => (
																	<FormItem className="w-full space-y-2">
																		<FormLabel className="text-xs/5 text-neutral-9">
																			Password
																		</FormLabel>
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
																				{...field}
																			/>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													</div>
												)}

												{outgoingForm.watch('pub_sub.kafka.auth.type') ==
													'scram' && (
													<div className="col-span-1">
														<FormField
															name="pub_sub.kafka.auth.hash"
															control={outgoingForm.control}
															render={({ field }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Hash
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
													</div>
												)}
											</div>
										</section>
									)}

									{outgoingForm.watch('pub_sub.type') == 'sqs' && (
										<section className="grid grid-cols-2 gap-x-5 gap-y-4">
											<h3 className="text-xs font-semibold col-span-full">
												Configure SQS
											</h3>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.sqs.access_key_id"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<div>
																<FormLabel className="text-xs/5 text-neutral-9">
																	AWS Access Key ID
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.sqs.secret_key"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																AWS Secret Access Key
															</FormLabel>
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
													name="pub_sub.sqs.default_region"
													control={outgoingForm.control}
													render={({ field }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																AWS Region
															</FormLabel>
															<Select
																name={field.name}
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
																	{AWSRegions.map(region => (
																		<SelectItem
																			className="cursor-pointer text-xs py-3 hover:bg-transparent"
																			value={region.uid}
																			key={region.uid}
																		>
																			<span className="text-neutral-10">
																				{region.name}
																			</span>
																		</SelectItem>
																	))}
																</SelectContent>
															</Select>
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.sqs.queue_name"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Queue Name
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</section>
									)}

									{outgoingForm.watch('pub_sub.type') == 'amqp' && (
										<section className="grid grid-cols-2 gap-x-5 gap-y-4">
											<h3 className="text-xs font-semibold col-span-full">
												Configure AMQP / RabbitMQ
											</h3>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.schema"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Schema
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.host"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Host
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.port"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Port
															</FormLabel>
															<FormControl>
																<Input
																	type="number"
																	inputMode="numeric"
																	pattern="\d*"
																	step={1}
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.queue"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Queue
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.deadLetterExchange"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Dead Letter Exchange
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
																			In case of failure, the message will be
																			published to the dlx, please note that
																			this will not declare the dlx.
																		</p>
																	</TooltipContent>
																</Tooltip>
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="col-span-1">
												<FormField
													control={outgoingForm.control}
													name="pub_sub.amqp.vhost"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-xs/5 text-neutral-9">
																Virtual Host
															</FormLabel>
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
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											{/* Checkboxes for Authentication and Bind Exchange */}
											<div className="flex items-center gap-x-4">
												<FormField
													control={outgoingForm.control}
													name="showAMQPAuth"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Authentication"
																	// @ts-expect-error the default value is boolean
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

												<FormField
													control={outgoingForm.control}
													name="showAMQPBindExhange"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Bind Exchange"
																	// @ts-expect-error the default value is boolean
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>
											</div>

											{/* AMQP Username/Password */}
											{outgoingForm.watch('showAMQPAuth') && (
												<div className="col-span-full grid grid-cols-2 gap-x-5 gap-y-4 pl-4 border-l border-new.primary-25">
													<h4 className="text-xs font-semibold col-span-full">
														Authentication
													</h4>
													<div>
														<FormField
															name="pub_sub.amqp.auth.user"
															control={outgoingForm.control}
															render={({ field, fieldState }) => (
																<FormItem className="w-full space-y-2">
																	<FormLabel className="text-xs/5 text-neutral-9">
																		Username
																	</FormLabel>
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
															name="pub_sub.amqp.auth.password"
															control={outgoingForm.control}
															render={({ field, fieldState }) => (
																<FormItem className="w-full space-y-2">
																	<FormLabel className="text-xs/5 text-neutral-9">
																		Password
																	</FormLabel>
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
																			{...field}
																		/>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												</div>
											)}

											{/* AMQP Binding Exchange*/}
											{outgoingForm.watch('showAMQPBindExhange') && (
												<div className="col-span-full grid grid-cols-2 gap-x-5 gap-y-4 pl-4 border-l border-new.primary-25">
													<h4 className="text-xs font-semibold col-span-full">
														Bind Exchange
													</h4>

													<div>
														<FormField
															name="pub_sub.amqp.bindExchange.exchange"
															control={outgoingForm.control}
															render={({ field, fieldState }) => (
																<FormItem className="w-full space-y-2">
																	<FormLabel className="text-xs/5 text-neutral-9">
																		Exchange
																	</FormLabel>
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
															name="pub_sub.amqp.bindExchange.routingKey"
															control={outgoingForm.control}
															render={({ field, fieldState }) => (
																<FormItem className="w-full space-y-2">
																	<FormLabel className="text-xs/5 text-neutral-9">
																		Routing Key
																	</FormLabel>
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
																			{...field}
																			// TODO default value is `""`
																		/>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												</div>
											)}
										</section>
									)}

									<hr />

									<div className="flex items-center gap-x-4">
										<FormField
											control={outgoingForm.control}
											name="showTransform"
											render={({ field }) => (
												<FormItem>
													<FormControl>
														<ConvoyCheckbox
															label="Transform"
															// @ts-expect-error the default value fixes this
															isChecked={field.value}
															onChange={field.onChange}
															disabled={
																!licenses.includes('WEBHOOK_TRANSFORMATIONS')
															}
														/>
													</FormControl>
												</FormItem>
											)}
										/>
									</div>

									{outgoingForm.watch('showTransform') && (
										<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
											<div className="flex flex-col gap-y-2 justify-center">
												<p className="text-neutral-10 text-xs">Transform</p>
												<p className="text-[10px] text-neutral-10">
													Transform request body of events with a JavaScript
													function.
												</p>
											</div>
											<div>
												<Button
													type="button"
													variant="outline"
													size="sm"
													disabled={
														!licenses.includes('WEBHOOK_TRANSFORMATIONS')
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
							{/* Submit Button */}
							<div className="flex justify-end mt-6 w-full">
								<Button
									type="submit"
									disabled={
										isUpdating ||
										!canManageSources ||
										!outgoingForm.formState.isValid
									}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isUpdating ? 'Updating...' : 'Update'} Source
									<ChevronRight className="stroke-white-100" />
								</Button>
							</div>
						</form>
					</Form>
				)}
			</section>

			{/* Reate Incoming Source Response Dialog */}
			<Dialog
				open={hasCreatedIncomingSource}
				onOpenChange={isOpen => {
					if (!isOpen) {
						return navigate({
							to: '/projects/$projectId/sources',
							params: { projectId: project.uid },
						});
					}
				}}
			>
				<DialogContent
					className="max-w-[480px] rounded-lg p-4"
					aria-describedby={undefined}
				>
					<DialogHeader>
						<DialogTitle className="text-base font-semibold text-start mb-4">
							Source URL
						</DialogTitle>
						<DialogDescription className="sr-only">
							Source URL created
						</DialogDescription>
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
				<DialogContent className="w-[90%] h-[90%] max-w-[90%] max-h-[90%] p-0 overflow-hidden rounded-8px">
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
									}}
									disabled={!isTransformPassed}
								>
									<SaveIcon className="stroke-white-100" />
									Save Function
								</Button>
							</div>
						</DialogHeader>

						{/* Dialog Body */}
						<div className="flex-1 overflow-auto p-6">
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
