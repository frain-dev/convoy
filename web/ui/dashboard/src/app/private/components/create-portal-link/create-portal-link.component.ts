import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
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

@Component({
	selector: 'convoy-create-portal-link',
	standalone: true,
	imports: [CommonModule, ModalComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, SelectComponent, CardComponent, ButtonComponent, ReactiveFormsModule, CopyButtonComponent, ModalHeaderComponent],
	templateUrl: './create-portal-link.component.html',
	styleUrls: ['./create-portal-link.component.scss']
})
export class CreatePortalLinkComponent implements OnInit {
	portalLinkForm: FormGroup = this.formBuilder.group({
		name: [null, Validators.required],
		endpoints: [null, Validators.required]
	});
	endpoints!: ENDPOINT[];
	isCreatingPortalLink = false;
	fetchingLinkDetails = false;
	portalLink!: string;
	linkUid = this.route.snapshot.params.id;

	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private generalService: GeneralService, private createPortalLinkService: CreatePortalLinkService, private router: Router, private route: ActivatedRoute) {}

	async ngOnInit() {
		this.getEndpoints();
		if (this.linkUid) await this.getPortalLink();
	}

	async savePortalLink() {
		this.isCreatingPortalLink = true;
		try {
			const response = this.linkUid ? await this.createPortalLinkService.updatePortalLink({ linkId: this.linkUid, data: this.portalLinkForm.value }) : await this.createPortalLinkService.createPortalLink({ data: this.portalLinkForm.value });

			this.generalService.showNotification({ message: response.message, style: 'success' });
			if (!this.linkUid) this.portalLink = response.data.url;
			if (this.linkUid) this.goBack();
			this.isCreatingPortalLink = false;
		} catch {
			this.isCreatingPortalLink = false;
		}
	}

	async getEndpoints(searchString?: string) {
		try {
			const response = await this.privateService.getEndpoints({ searchString });
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
			this.portalLinkForm.patchValue({
				name: linkDetails.name,
				endpoints: linkDetails.endpoints
			});
			this.fetchingLinkDetails = false;
		} catch {
			this.fetchingLinkDetails = false;
		}
	}

	goBack() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/portal-links');
	}
}
