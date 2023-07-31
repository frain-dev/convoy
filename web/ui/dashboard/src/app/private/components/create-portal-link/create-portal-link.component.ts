import { Component, OnInit, inject, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { CardComponent } from 'src/app/components/card/card.component';
import { PrivateService } from '../../private.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute, Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CreatePortalLinkService } from './create-portal-link.service';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';

@Component({
	selector: 'convoy-create-portal-link',
	standalone: true,
	imports: [CommonModule, DialogHeaderComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, SelectComponent, CardComponent, ButtonComponent, ReactiveFormsModule, CopyButtonComponent, RadioComponent, ToggleComponent],
	templateUrl: './create-portal-link.component.html',
	styleUrls: ['./create-portal-link.component.scss']
})
export class CreatePortalLinkComponent implements OnInit {
	@Input('action') action?: 'create' | 'update';
	portalLinkForm: FormGroup = this.formBuilder.group({
		name: [null, Validators.required],
		endpoints: [null, Validators.required],
		owner_id: [null, Validators.required],
		can_manage_endpoint: [false, Validators.required],
		type: [null, Validators.required]
	});
	endpoints!: ENDPOINT[];
	isCreatingPortalLink = false;
	fetchingLinkDetails = false;
	portalLink!: string;
	linkUid = this.route.snapshot.params.id;
	private rbacService = inject(RbacService);

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private generalService: GeneralService, private createPortalLinkService: CreatePortalLinkService, private router: Router, private route: ActivatedRoute) {}

	async ngOnInit() {
		this.getEndpoints();
		if (this.action === 'update') await this.getPortalLink();
		if (!(await this.rbacService.userCanAccess('Portal Links|MANAGE'))) this.portalLinkForm.disable();
	}

	async savePortalLink() {
		this.isCreatingPortalLink = true;

		try {
			this.portalLinkForm.patchValue(this.portalLinkForm.value.type == 'endpoint' ? { owner_id: null } : { endpoints: null });
			const portalDetails = structuredClone(this.portalLinkForm.value);
			delete portalDetails.type;

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

	async getEndpoints(searchString?: string) {
		try {
			const response = await this.privateService.getEndpoints({ q: searchString });
			const endpointData = response.data.content;
			endpointData.forEach((data: ENDPOINT) => {
				data.name = data.title;
			});
			this.endpoints = endpointData;
		} catch {}
	}

	async getPortalLink() {
		this.fetchingLinkDetails = true;

		try {
			const response = await this.createPortalLinkService.getPortalLink(this.linkUid);
			const linkDetails = response.data;
			this.portalLinkForm.patchValue({ ...linkDetails, type: linkDetails.endpoints ? 'endpoint' : 'owner_id' });
			this.fetchingLinkDetails = false;
		} catch {
			this.fetchingLinkDetails = false;
		}
	}

	goBack() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/portal-links');
	}
}
