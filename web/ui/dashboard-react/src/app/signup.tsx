import { z } from 'zod';
import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, type UseFormReturn } from 'react-hook-form';
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router';

import { EyeIcon, EyeOffIcon } from 'lucide-react';

import {
	Form,
	FormControl,
	FormField,
	FormItem,
	FormLabel,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

import { cn } from '@/lib/utils';
import * as authService from '@/services/auth.service';
import { CONVOY_DASHBOARD_DOMAIN } from '@/lib/constants';
import * as hubSpotService from '@/services/hubspot.service';
import * as licensesService from '@/services/licenses.service';

export const Route = createFileRoute('/signup')({
	async beforeLoad() {
		try {
			const licenses = await licensesService.getLicenses();
			if (!licenses.includes('CREATE_USER')) {
				throw new Error('beforeLoad: client is not licensed to create user');
			}
		} catch (err) {
			console.error('SignUpPage.beforeLoad:', err);
			redirect({ to: '/', throw: true });
		}
	},

	component: SignUpPage,
});

type FormFieldInputComponentProps = {
	form: UseFormReturn<z.infer<typeof formSchema>>;
};

function BusinessNameInputField({ form }: FormFieldInputComponentProps) {
	return (
		<FormField
			control={form.control}
			name="orgName"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center">
						<FormLabel className="w-full flex items-center justify-between">
							<span className="text-xs/5 text-neutral-9 block">
								Business Name
							</span>
						</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="organization"
							type="text"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-destructive focus-visible:ring-0 hover:border-destructive'
									: ' hover:border-new.primary-100 focus:border-new.primary-300',
							)}
							placeholder="Convoy"
							{...field}
						/>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function EmailInputField({ form }: FormFieldInputComponentProps) {
	return (
		<FormField
			control={form.control}
			name="email"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center">
						<FormLabel className="w-full flex items-center justify-between">
							<span className="text-xs/5 text-neutral-9 block">Email</span>
						</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="email"
							type="email"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-destructive focus-visible:ring-0 hover:border-destructive'
									: ' hover:border-new.primary-100 focus:border-new.primary-300',
							)}
							placeholder="super@default.com"
							{...field}
						/>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function FirstNameInputField({ form }: FormFieldInputComponentProps) {
	return (
		<FormField
			control={form.control}
			name="firstName"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center">
						<FormLabel className="w-full flex items-center justify-between">
							<span className="text-xs/5 text-neutral-9 block">First Name</span>
						</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="name"
							type="text"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-destructive focus-visible:ring-0 hover:border-destructive'
									: ' hover:border-new.primary-100 focus:border-new.primary-300',
							)}
							placeholder="John"
							{...field}
						/>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function LastNameInputField({ form }: FormFieldInputComponentProps) {
	return (
		<FormField
			control={form.control}
			name="lastName"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center">
						<FormLabel className="w-full flex items-center justify-between">
							<span className="text-xs/5 text-neutral-9 block">Last Name</span>
						</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="family-name"
							type="text"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-destructive focus-visible:ring-0 hover:border-destructive'
									: ' hover:border-new.primary-100 focus:border-new.primary-300',
							)}
							placeholder="Doe"
							{...field}
						/>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function PasswordInputField({ form }: FormFieldInputComponentProps) {
	const [isPasswordVisible, setIsPasswordVisible] = useState(false);

	return (
		<FormField
			control={form.control}
			name="password"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-[8px] flex items-center justify-between">
						<FormLabel
							className="text-xs/5 text-neutral-9"
							htmlFor="signup_password_input"
						>
							Password
						</FormLabel>
					</div>
					<FormControl className="w-full relative">
						<div className="relative">
							<Input
								id="signup_password_input"
								autoComplete="current-password"
								type={isPasswordVisible ? 'text' : 'password'}
								className={cn(
									'hide-password-toggle mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
									fieldState.error
										? 'border-destructive focus-visible:ring-0 hover:border-destructive'
										: 'hover:border-new.primary-100 focus:border-new.primary-300',
								)}
								{...field}
							/>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								className="absolute right-[1%] top-0 h-full px-3 py-2 hover:bg-transparent"
								onClick={() => setIsPasswordVisible(!isPasswordVisible)}
							>
								{isPasswordVisible ? (
									<EyeIcon className="opacity-50" aria-hidden="true" />
								) : (
									<EyeOffIcon className="opacity-50" aria-hidden="true" />
								)}
								<span className="sr-only">
									{isPasswordVisible ? 'Hide password' : 'Show password'}
								</span>
							</Button>
						</div>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function SignUpButton(props: { isButtonEnabled?: boolean }) {
	const { isButtonEnabled } = props;

	return (
		<Button
			disabled={!isButtonEnabled}
			variant={'ghost'}
			size={'lg'}
			className="flex items-center justify-center disabled:opacity-50 cursor-pointer mb-3 bg-new.primary-400 hover:bg-new.primary-400 px-9 py-[10px] rounded-8px text-sm/5 text-white-100 w-full"
		>
			<span
				className={cn(
					'text-sm text-white-100',
					!isButtonEnabled ? 'hidden' : '',
				)}
			>
				Sign Up
			</span>
			<img
				className={cn('h-4', !isButtonEnabled ? '' : 'hidden')}
				src="assets/img/button-loader.gif"
				alt="loader"
			/>
		</Button>
	);
}

function SignUpWithSAMLButton() {
	async function signUp() {
		localStorage.setItem('AUTH_TYPE', 'signup'); // I don't know why we're doing this

		try {
			const { data } = await authService.signUpWithSAML();
			window.open(data.redirectUrl, '_blank');
		} catch (err) {
			// TODO show user on the UI
			console.error(err);
			throw err;
		}
	}

	return (
		<Button
			type="button"
			size={'lg'}
			variant={'ghost'}
			onClick={signUp}
			className="disabled:opacity-50 cursor-pointer w-full rounded-8px text-xs/5 text-new.primary-400 hover:text-new.primary-400 py-0 hover:bg-transparent h-auto"
		>
			Sign Up with SAML
		</Button>
	);
}

const formSchema = z.object({
	email: z.string().email('Please enter your email'),
	password: z.string().min(1, 'Password is required'),
	lastName: z.string().trim().min(1, 'Please enter your last name'),
	firstName: z.string().trim().min(1, 'Please enter your first name'),
	orgName: z.string().trim().min(1, 'Please enter your business name'),
});

function SignUpPage() {
	const navigate = useNavigate();
	const [isSignUpButtonEnabled, setIsSignUpButtonEnabled] = useState(true);

	const form = useForm<z.infer<typeof formSchema>>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			email: '',
			orgName: '',
			password: '',
			firstName: '',
			lastName: '',
		},
		mode: 'onTouched',
	});

	async function signUp(values: z.infer<typeof formSchema>) {
		setIsSignUpButtonEnabled(false);
		const { email, firstName, lastName, orgName, password } = values;

		try {
			await authService.signUp({
				email,
				password,
				org_name: orgName,
				last_name: lastName,
				first_name: firstName,
			});

			if (location.hostname == CONVOY_DASHBOARD_DOMAIN) {
				await hubSpotService.sendWelcomeEmail({
					email,
					firstname: firstName,
					lastname: lastName,
				});
			}

			setIsSignUpButtonEnabled(false);
			navigate({
				from: '/signup',
				to: '/get-started',
			});
		} catch (err) {
			// TODO show user error on UI
			setIsSignUpButtonEnabled(false);
		}
	}

	function navigateToLoginPage() {
		return navigate({
			from: '/signup',
			to: '/login',
		});
	}

	return (
		<div className="flex w-full">
			<aside className="bg-primary-100 bg-[url('/assets/img/public-layout.png')] bg-no-repeat bg-right-top min-w-[374px] desktop:w-0 h-screen transition-all duration-300 px-[36px] pt-[70px]"></aside>
			<div className="bg-[url('/assets/svg/pattern.svg')] bg-center bg-cover min-h-screen w-full">
				<div className="min-h-screen flex flex-col items-center justify-center w-full">
					<img
						src="/assets/svg/logo.svg"
						alt="convoy logo"
						className="mb-7 w-[130px]"
					/>

					<section className="max-w-[445px] mx-auto my-0 p-6 w-full bg-white-100 shadow-default rounded-[8px]">
						<Form {...form}>
							<form
								onSubmit={(...args) => void form.handleSubmit(signUp)(...args)}
							>
								<BusinessNameInputField form={form} />
								<EmailInputField form={form} />
								<FirstNameInputField form={form} />
								<LastNameInputField form={form} />
								<PasswordInputField form={form} />
								<SignUpButton isButtonEnabled={isSignUpButtonEnabled} />
								<SignUpWithSAMLButton />
							</form>
						</Form>
					</section>

					<Button
						type="button"
						size={'sm'}
						variant={'ghost'}
						onClick={navigateToLoginPage}
						className="mt-[34px] disabled:opacity-50 cursor-pointer w-full rounded-8px text-xs/5 text-new.primary-400 hover:text-new.primary-400 py-0 hover:bg-transparent h-auto underline"
					>
						Log in if you already have an account
					</Button>
				</div>
			</div>
		</div>
	);
}
