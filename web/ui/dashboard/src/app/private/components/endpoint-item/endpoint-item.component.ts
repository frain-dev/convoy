import { CommonModule } from '@angular/common';
import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { PrivateService } from '../../private.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { ActivatedRoute, RouterModule } from '@angular/router';
import { EndpointSecretComponent } from '../../pages/project/endpoints/endpoint-secret/endpoint-secret.component';
import { DeleteModalComponent } from '../delete-modal/delete-modal.component';
import { EndpointsService } from '../../pages/project/endpoints/endpoints.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { TableCellComponent, TableRowComponent } from 'src/app/components/table/table.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DropdownContainerComponent } from 'src/app/components/dropdown-container/dropdown-container.component';

@Component({
	standalone: true,
	selector: 'convoy-endpoint, [convoy-endpoint]',
	templateUrl: './endpoint-item.component.html',
	imports: [CommonModule, ButtonComponent, TagComponent, CopyButtonComponent, StatusColorModule, RouterModule, EndpointSecretComponent, DeleteModalComponent, TableRowComponent, TableCellComponent, DropdownComponent, DropdownContainerComponent, DropdownOptionDirective]
})
export class EndpointComponent implements OnInit {
	@Input('endpoint') endpoint!: ENDPOINT;
	@Input('endpoints') endpoints: ENDPOINT[] = [];
	@Input('i') i!: number;
	@Output('clear') clearEndpoint = new EventEmitter<any>();
	@Output('set') setEndpoint = new EventEmitter<any>();
	@Output('getEndpoints') getEndpoints = new EventEmitter<any>();
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;
	loadingFilterEndpoints = false;
	selectedEndpoint?: ENDPOINT;
	isTogglingEndpoint = false;
	isDeletingEndpoint = false;
	isSendingTestEvent = false;

	constructor(public privateService: PrivateService, public route: ActivatedRoute, private endpointService: EndpointsService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	ngAfterViewInit() {}

	async getEndpointsForFilter(search: string): Promise<ENDPOINT[]> {
		return await (
			await this.privateService.getEndpoints({ q: search })
		).data.content;
	}

	clear() {
		this.clearEndpoint.emit();
	}

	set() {
		this.setEndpoint.emit(this.selectedEndpoint);
	}

	async toggleEndpoint() {
		this.isTogglingEndpoint = true;
		if (!this.selectedEndpoint?.uid) return;

		try {
			const response = await this.endpointService.toggleEndpoint(this.selectedEndpoint?.uid);
			this.endpoint.status = response.data.status;
			this.generalService.showNotification({ message: `${this.selectedEndpoint?.title} status updated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	async sendTestEvent() {
		const testEvent = {
			data: { data: 'test event from Convoy', convoy: 'https://getconvoy.io', amount: 1000 },
			endpoint_id: this.selectedEndpoint?.uid,
			event_type: 'test.convoy'
		};

		this.isSendingTestEvent = true;
		try {
			const response = await this.endpointService.sendEvent({ body: testEvent });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isSendingTestEvent = false;
		} catch {
			this.isSendingTestEvent = false;
		}
	}

	async deleteEndpoint() {
		if (!this.selectedEndpoint) return;
		this.isDeletingEndpoint = true;

		try {
			const response = await this.endpointService.deleteEndpoint(this.selectedEndpoint?.uid || '');
			this.getEndpoints.emit({ hideLoader: true });

			this.generalService.showNotification({ style: 'success', message: response.message });
			this.deleteDialog.nativeElement.close();
			this.isDeletingEndpoint = false;
		} catch {
			this.isDeletingEndpoint = false;
		}
	}
}
