import { z } from 'zod';
import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, Link } from '@tanstack/react-router';

import { CopyIcon } from 'lucide-react';

import { Form } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { DashboardLayout } from '@/components/dashboard';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from '@/components/ui/dialog';

import { cn } from '@/lib/utils';
import { useOrganisationStore } from '@/store';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as authService from '@/services/auth.service';
import * as orgsService from '@/services/organisations.service';

import warningAnimation from '../../assets/img/warning-animation.gif';

import type { Organisation } from '@/models/organisation.model';

type SettingsSearch = {
	token: string | undefined;
};

export const Route = createFileRoute('/settings')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	validateSearch: (search: Record<string, unknown>): SettingsSearch => {
		return {
			token: (search.token as string) || undefined,
		};
	},
	loaderDeps: ({ search: { token } }) => ({ token }),
	async loader({ deps }) {
		const canManageOrganisation = await authService.ensureUserCanAccess(
			'Organisations|MANAGE',
			deps.token,
		);

		return {
			canManageOrganisation,
		};
	},
	component: SettingsPage,
});

const OrganisationFormSchema = z.object({
	orgName: z.string().min(1, 'Please enter a name for your organisation'),
	orgId: z.string(),
});

function SettingsPage() {
	const { canManageOrganisation } = Route.useLoaderData();
	const [isUpdatingOrg, setIsUpdatingOrg] = useState(false);
	const [isDeletingOrg, setIsDeletingOrg] = useState(false);
	const { org, setPaginatedOrgs, setOrg, paginatedOrgs } =
		useOrganisationStore();

	const organisationForm = useForm<z.infer<typeof OrganisationFormSchema>>({
		resolver: zodResolver(OrganisationFormSchema),
		defaultValues: {
			orgId: org?.uid,
			orgName: org?.name,
		},
		mode: 'onTouched',
	});

	useEffect(() => {
		organisationForm.setValue('orgId', org?.uid || '');
		organisationForm.setValue('orgName', org?.name || '');
	}, [org]);

	async function updateOrganisation(
		values: z.infer<typeof OrganisationFormSchema>,
	) {
		if (org?.name == values.orgName) return;

		setIsUpdatingOrg(true);
		try {
			const org = await orgsService.updateOrganisation({
				name: values.orgName.trim(),
				orgId: values.orgId,
			});
			setOrg(org);
			setPaginatedOrgs({
				pagination: paginatedOrgs.pagination,
				content: paginatedOrgs.content.reduce(
					(acc: Array<Organisation>, _org) => {
						if (org.uid != _org.uid) return acc.concat(_org);
						_org.name = org.name;
						return acc.concat(_org);
					},
					[],
				),
			});
			// TODO toast message telling user is successful
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdatingOrg(false);
		}
	}

	async function deleteOrganisation() {
		setIsDeletingOrg(true);
		try {
			await orgsService.deleteOrganisation(org?.uid || '');

			setPaginatedOrgs({
				pagination: paginatedOrgs.pagination,
				content: paginatedOrgs.content.filter((_org) => org?.uid != _org.uid),
			});

			setOrg(paginatedOrgs.content.filter((_org) => org?.uid != _org.uid)[0] || null);
		} catch (error) {
			console.error(error);
		} finally {
			setIsDeletingOrg(false);
		}
	}

	return (
		<DashboardLayout showSidebar={false}>
			<div className="flex justify-start items-center gap-2">
				<Link
					to="/projects"
					className="block p-[2px] rounded-[100%] border border-new.primary-5"
				>
					<svg width="24" height="24" className="fill-neutral-10 scale-75">
						<use xlinkHref="#arrow-left-icon"></use>
					</svg>
				</Link>
				<h1 className="font-semibold text-xs text-neutral-12">
					Organisation Settings
				</h1>
			</div>
			<Tabs
				defaultValue="organisation"
				activationMode="manual"
				orientation="vertical"
				className="flex w-full"
			>
				<TabsList className="">
					<div className="flex flex-col items-start space-y-2">
						<TabsTrigger value="organisation">Organisation</TabsTrigger>
						<TabsTrigger value="team">Team</TabsTrigger>
					</div>
				</TabsList>
				<TabsContent value="organisation">
					<section className="w-full">
						<Form {...organisationForm}>
							<form
								onSubmit={(...args) =>
									void organisationForm.handleSubmit(updateOrganisation)(
										...args,
									)
								}
							>
								<div className="flex justify-between items-center mb-7">
									<h2 className="text-base font-semibold">Organisation</h2>
									<Button
										disabled={isUpdatingOrg || !canManageOrganisation || !org}
										size="sm"
										variant="ghost"
										className="px-4 py-2 text-xs bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100"
									>
										Save Changes
									</Button>
								</div>

								<FormField
									control={organisationForm.control}
									name="orgName"
									render={({ field, fieldState }) => (
										<FormItem className="w-full relative mb-6 block">
											<div className="w-full mb-2 flex items-center justify-between">
												<FormLabel className="text-xs/5 text-neutral-9">
													Organisation name
												</FormLabel>
											</div>
											<FormControl>
												<Input
													disabled={!canManageOrganisation}
													autoComplete="organization"
													type="text"
													className={cn(
														'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
														fieldState.error
															? 'border-destructive focus-visible:ring-0 hover:border-destructive'
															: ' hover:border-new.primary-100 focus:border-new.primary-300',
													)}
													placeholder="Organisation name"
													{...field}
												/>
											</FormControl>
											<FormMessageWithErrorIcon />
										</FormItem>
									)}
								/>

								<FormField
									control={organisationForm.control}
									name="orgId"
									render={({ field }) => (
										<FormItem className="w-full relative mb-6 block">
											<div className="w-full mb-2 flex items-center justify-between">
												<FormLabel className="text-xs/5 text-neutral-9">
													Organisation ID
												</FormLabel>
											</div>
											<FormControl>
												<div className="relative">
													<Input
														readOnly
														autoComplete="off"
														type="text"
														className={cn(
															'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] font-normal',
														)}
														{...field}
													/>
													<Button
														type="button"
														variant="ghost"
														size="sm"
														className="absolute right-[1%] top-0 h-full px-3 py-2 hover:bg-transparent"
														onClick={() => {
															window.navigator.clipboard
																.writeText(org?.uid || '')
																.then();
															// TODO show toast message on copy successful
														}}
													>
														<CopyIcon
															className="opacity-50"
															aria-hidden="true"
														/>
														<span className="sr-only">
															copy organisation id
														</span>
													</Button>
												</div>
											</FormControl>
											<FormMessageWithErrorIcon />
										</FormItem>
									)}
								/>
							</form>
						</Form>
					</section>
					<hr className="my-10" />
					<section className="bg-destructive/5 border-destructive/30 border p-6 rounded-8px flex flex-col items-start justify-center">
						<h2 className="text-destructive font-semibold text-lg mb-5">
							Danger Zone
						</h2>
						<p className="text-sm mb-8">
							Deleting your organisation means you will lose all workspaces
							created by you and all your every other organisation information.
						</p>
						<Dialog>
							<DialogTrigger asChild>
								<Button
									disabled={isDeletingOrg || !canManageOrganisation || !org}
									size="sm"
									variant="ghost"
									className="px-4 py-2 text-xs bg-destructive  hover:bg-destructive hover:text-white-100 flex items-center"
								>
									<svg width="18" height="18" className="fill-white-100">
										<use xlinkHref="#delete-icon"></use>
									</svg>
									<p className="text-white-100">Delete Organisation</p>
								</Button>
							</DialogTrigger>
							<DialogContent className="sm:max-w-[432px] rounded-lg">
								<DialogHeader>
									<DialogTitle className="flex justify-center items-center">
										<img
											src={warningAnimation}
											alt="warning"
											className="w-24"
										/>
									</DialogTitle>
									<DialogDescription className="flex justify-center items-center font-medium text-new.black text-sm">
										Are you sure you want to deactivate “{org?.name}”?
									</DialogDescription>
								</DialogHeader>
								<div className="flex flex-col items-center space-y-4">
									<p className="text-xs text-neutral-11">
										This action is irreversible.
									</p>
									<DialogClose asChild>
										<Button
											onClick={deleteOrganisation}
											type="submit"
											size="sm"
											className="bg-destructive text-white-100 hover:bg-destructive hover:text-white-100"
										>
											Yes. Deactivate
										</Button>
									</DialogClose>
								</div>
								<DialogFooter className="flex justify-center items-center">
									<DialogClose asChild>
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
					</section>
				</TabsContent>
				<TabsContent value="team">
					<div className="w-full">
						<p className="text-xl">Team</p>
					</div>
				</TabsContent>
			</Tabs>
		</DashboardLayout>
	);
}
