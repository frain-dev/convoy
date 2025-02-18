import { Button } from '@/components/ui/button';
import {
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Form } from '@/components/ui/form';
import { cn } from '@/lib/utils';
import { zodResolver } from '@hookform/resolvers/zod';
import { EyeIcon, EyeOffIcon } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import type { UseFormReturn } from 'react-hook-form';

const formSchema = z.object({
	username: z.string().min(1, 'Please enter your username'),
	password: z.string().min(1, 'Please enter your password'),
});

type UsernameInputFieldProps = {
	form: UseFormReturn<z.infer<typeof formSchema>>;
};

type PasswordInputFieldProps = {
	form: UseFormReturn<z.infer<typeof formSchema>>;
	showPassword: boolean;
	setShowPassword: React.Dispatch<React.SetStateAction<boolean>>;
};

function UsernameInputField({ form }: UsernameInputFieldProps) {
	return (
		<FormField
			control={form.control}
			name="username"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center justify-between">
						<FormLabel className="text-xs/5 text-neutral-9">Username</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="username"
							type="text"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-full transition-all duration-300 bg-white-100 py-[14.5px] px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-new.error-500 focus-visible:ring-0 hover:border-new.error-500'
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

function PasswordInputField(props: PasswordInputFieldProps) {
	const { form, showPassword, setShowPassword } = props;
	return (
		<FormField
			control={form.control}
			name="password"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-2 block">
					<div className="w-full mb-[8px] flex items-center justify-between">
						<FormLabel
							className="text-xs/5 text-neutral-9"
							htmlFor="password_input"
						>
							Password
						</FormLabel>
					</div>
					<FormControl className="w-full relative">
						<div className="relative">
							<Input
								id="password_input"
								autoComplete="current-password"
								type={showPassword ? 'text' : 'password'}
								className={cn(
									'hide-password-toggle mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-full transition-all duration-300 bg-white-100 py-[14.5px] px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
									fieldState.error
										? 'border-new.error-500 focus-visible:ring-0 hover:border-new.error-500'
										: 'hover:border-new.primary-100 focus:border-new.primary-300',
								)}
								placeholder="super@default.com"
								{...field}
							/>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								className="absolute right-[1%] top-0 h-full px-3 py-2 hover:bg-transparent"
								onClick={() => {
									setShowPassword(prev => !prev);
								}}
							>
								{showPassword ? (
									<EyeIcon className="opacity-50" aria-hidden="true" />
								) : (
									<EyeOffIcon className="opacity-50" aria-hidden="true" />
								)}
								<span className="sr-only">
									{showPassword ? 'Hide password' : 'Show password'}
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

function ForgotPasswordSection() {
	function navigateToPasswordPage() {
		console.log('router.navigateByUrl("/forgot-password")');
	}

	return (
		<div className="flex items-center text-xs/5 mb-[20px]">
			Forgot password?
			<Button
				variant={'link'}
				type="button"
				className="p-2 ml-[16px] text-new.primary-400 underline underline-offset-1"
				onClick={navigateToPasswordPage}
				size="sm"
			>
				Reset it here
			</Button>
		</div>
	);
}

function LoginButton(props: { disableLoginButton: boolean }) {
	const { disableLoginButton } = props;

	return (
		<Button
			disabled={disableLoginButton}
			variant={'ghost'}
			size={'lg'}
			className="flex items-center justify-center disabled:opacity-50 cursor-pointer mb-3 bg-new.primary-400 hover:bg-new.primary-400 px-9 py-[10px] rounded-8px text-sm/5 text-white-100 w-full"
		>
			<span
				className={cn(
					'text-sm text-white-100',
					disableLoginButton ? 'hidden' : '',
				)}
			>
				Login
			</span>
			<img
				className={cn('h-4', disableLoginButton ? '' : 'hidden')}
				src="assets/img/button-loader.gif"
				alt="loader"
			/>
		</Button>
	);
}

function LoginWithSAMLButton() {
	function login() {
		console.log('login with SAML');
	}

	return (
		<Button
			type="button"
			size={'lg'}
			variant={'ghost'}
			onClick={login}
			className="disabled:opacity-50 cursor-pointer w-full rounded-8px text-xs/5 text-new.primary-400 hover:text-new.primary-400 py-0 hover:bg-transparent h-auto"
		>
			Login with SAML
		</Button>
	);
}

export function LoginPage() {
	const [disableLoginButton] = useState<boolean>(false);
	const [showPassword, setShowPassword] = useState<boolean>(false);

	const form = useForm<z.infer<typeof formSchema>>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			username: '',
			password: '',
		},
		mode: 'onTouched',
	});

	function login(values: z.infer<typeof formSchema>) {
		console.log(values);
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
								onSubmit={(...args) => void form.handleSubmit(login)(...args)}
							>
								<UsernameInputField form={form} />

								<PasswordInputField
									form={form}
									showPassword={showPassword}
									setShowPassword={setShowPassword}
								/>

								<ForgotPasswordSection />

								<LoginButton disableLoginButton={disableLoginButton} />

								<LoginWithSAMLButton />
							</form>
						</Form>
					</section>
				</div>
			</div>
		</div>
	);
}
