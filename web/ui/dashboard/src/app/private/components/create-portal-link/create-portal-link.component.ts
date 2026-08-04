import { Component, inject, Input, OnInit } from '@angular/core';

import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { PrivateService } from '../../private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute, Router } from '@angular/router';
import { CreatePortalLinkService } from './create-portal-link.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { NotificationComponent } from 'src/app/components/notification/notification.component';

@Component({
    selector: 'convoy-create-portal-link',
    imports: [ReactiveFormsModule, NotificationComponent],
    templateUrl: './create-portal-link.component.html',
    styleUrls: ['./create-portal-link.component.scss']
})
export class CreatePortalLinkComponent implements OnInit {
	@Input('action') action?: 'create' | 'update';
	portalLinkForm: FormGroup = this.formBuilder.group({
		name: [null, Validators.required],
		owner_id: [null, Validators.required],
		can_manage_endpoint: [false],
		auth_type: ['refresh_token', Validators.required]
	});
	authTypeOptions = [
		{ value: 'static_token', label: 'Static Token', description: 'Default authentication type for existing portal links.' },
		{ value: 'refresh_token', label: 'Refresh Token', description: 'Short-lived token that can be used to access the portal link.' }
	];
	isCreatingPortalLink = false;
	fetchingLinkDetails = false;
	portalLink!: string;
	linkUid = this.route.snapshot.params.id;
	private rbacService = inject(RbacService);

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private generalService: GeneralService, private createPortalLinkService: CreatePortalLinkService, private router: Router, private route: ActivatedRoute) {}

	async ngOnInit() {
		if (this.action === 'update') await this.getPortalLink();
		if (!(await this.rbacService.userCanAccess('Portal Links|MANAGE'))) this.portalLinkForm.disable();
	}

	async savePortalLink() {
		if (this.portalLinkForm.invalid) {
			this.portalLinkForm.markAllAsTouched();
			return;
		}

		this.isCreatingPortalLink = true;

		try {
			const portalDetails = structuredClone(this.portalLinkForm.value);

			const response = this.action === 'update' ? await this.createPortalLinkService.updatePortalLink({ linkId: this.linkUid, data: portalDetails }) : await this.createPortalLinkService.createPortalLink({ data: portalDetails });

			this.generalService.showNotification({ message: response.message, style: 'success' });
			if (this.action === 'create') {
				this.portalLink = response.data.url;
				this.portalLinkForm.disable();
			}
			if (this.action === 'update') this.goBack();
			this.isCreatingPortalLink = false;
		} catch {
			this.isCreatingPortalLink = false;
		}
	}

	async getPortalLink() {
		this.fetchingLinkDetails = true;

		try {
			const response = await this.createPortalLinkService.getPortalLink(this.linkUid);
			const linkDetails = response.data;
			this.portalLinkForm.patchValue({ ...linkDetails });
			this.fetchingLinkDetails = false;
		} catch {
			this.fetchingLinkDetails = false;
		}
	}

	copyLink() {
		if (!this.portalLink) return;
		navigator.clipboard.writeText(this.portalLink).then(() => {
			this.generalService.showNotification({ message: 'URL has been copied to clipboard', style: 'info' });
		});
	}

	goBack() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/portal-links');
	}
}
