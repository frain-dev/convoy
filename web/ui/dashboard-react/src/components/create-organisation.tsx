import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogClose,
	DialogTrigger,
} from '@/components/ui/dialog';
import {
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
	Form,
} from '@/components/ui/form';

import { cn } from '@/lib/utils';
import * as orgsService from '@/services/organisations.service';

import organisationIcon from '../../assets/svg/organisation-icon.svg';
import orgEmptyStateImg from '../../assets/svg/organizations-empty-state.svg';

import type { ReactNode, SetStateAction } from 'react';

const FormSchema = z.object({
	orgName: z.string().min(1, 'Organisation name is required'),
});

type CreateOrganisationDialogProps = {
	trigger: ReactNode;
	setIsDialogOpen: (v: SetStateAction<boolean>) => void;
	isDialogOpen: boolean;
	onOrgCreated: () => void;
};

export function CreateOrganisationDialog(props: CreateOrganisationDialogProps) {
	const form = useForm<z.infer<typeof FormSchema>>({
		resolver: zodResolver(FormSchema),
		defaultValues: {
			orgName: '',
		},
		mode: 'onTouched',
	});

	async function createOrganisation(values: z.infer<typeof FormSchema>) {
		try {
			await orgsService.addOrganisation({ name: values.orgName });
			props.onOrgCreated();
		} catch (error) {
			console.error(error);
			// TODO show toast message on all catch(error)s where necessary
		} finally {
			props.setIsDialogOpen(isOpen => !isOpen);
		}
	}

	return (
		// TODO remove the X button
		<Dialog open={props.isDialogOpen}>
			<DialogTrigger asChild>
				{props.trigger}
			</DialogTrigger>
			<DialogContent className="sm:max-w-[432px] rounded-lg">
				<DialogHeader>
					<DialogTitle className="text-left py-3">
						<img src={organisationIcon} className="w-16" alt="organisation" />
					</DialogTitle>
					<DialogDescription className="text-sm text-start">
						Your organisation information will help us to know how to get you
						set up.
					</DialogDescription>
				</DialogHeader>
				<div className="grid gap-4 py-4">
					<Form {...form}>
						<form
							onSubmit={(...args) =>
								void form.handleSubmit(createOrganisation)(...args)
							}
						>
							<FormField
								control={form.control}
								name="orgName"
								render={({ field, fieldState }) => (
									<FormItem className="w-full relative mb-6 block">
										<div className="w-full mb-2 flex items-center justify-between">
											<FormLabel className="text-xs/5 text-neutral-9">
												What is your business's name?
											</FormLabel>
										</div>
										<FormControl>
											<Input
												autoComplete="name"
												type="text"
												className={cn(
													'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
													fieldState.error
														? 'border-new.error-500 focus-visible:ring-0 hover:border-new.error-500'
														: ' hover:border-new.primary-100 focus:border-new.primary-300',
												)}
												placeholder="e.g Kuda"
												{...field}
											/>
										</FormControl>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>
							<div className="flex items-center justify-end gap-4">
								<DialogClose asChild>
									<Button
										onClick={() => props.setIsDialogOpen(isOpen => !isOpen)}
										type="button"
										variant="outline"
										size={'sm'}
										className="hover:bg-white-100 border-destructive text- hover:text-destructive text-destructive shadow-none px-5 py-2"
									>
										Cancel
									</Button>
								</DialogClose>
								<Button
									size={'sm'}
									type="submit"
									className="bg-new.primary-400 hover:bg-new.primary-400 text-white-100 shadow-none hover:text-white-100 px-5 py-2"
									variant={'ghost'}
								>
									Create
								</Button>
							</div>
						</form>
					</Form>
				</div>
			</DialogContent>
		</Dialog>
	);
}

type CreateOrganisationProps = {
	onOrgCreated: () => void;
	children: ReactNode;
	setIsDialogOpen: (v: SetStateAction<boolean>) => void;
	isDialogOpen: boolean;
};

export function CreateOrganisation(props: CreateOrganisationProps) {
	return (
		<div className="flex flex-col items-center">
			<img
				className="h-40 my-12"
				src={orgEmptyStateImg}
				alt="no organisations created"
			/>
			<h2 className="font-bold text-base text-neutral-12 text-center mb-4">
				Create an organisation to get started with Convoy
			</h2>
			<p className="text-neutral-10 text-sm text-center mt-2">
				An organization is required to create projects on Convoy.
			</p>

			<CreateOrganisationDialog {...props} trigger={props.children}/>
		</div>
	);
}
