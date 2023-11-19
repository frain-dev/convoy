import { CommonModule } from '@angular/common';
import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { Observable, fromEvent } from 'rxjs';
import { map, startWith, debounceTime, distinctUntilChanged, switchMap } from 'rxjs/operators';
import { DropdownContainerComponent } from 'src/app/components/dropdown-container/dropdown-container.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { PrivateService } from '../../private.service';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	standalone: true,
	selector: 'convoy-endpoint-filter',
	templateUrl: './endpoints-filter.component.html',
	imports: [CommonModule, DropdownComponent, DropdownContainerComponent, DropdownOptionDirective, ListItemComponent, SkeletonLoaderComponent, ButtonComponent]
})
export class EndpointFilterComponent implements OnInit {
	@ViewChild('endpoint', { static: true }) eventDelsEndpointFilter!: ElementRef;
	@Input('endpoint') endpointId!: string | undefined;
	@Input('show') show: boolean = false;
	@Input('position') position: 'right' | 'left' | 'center' | 'right-side' = 'left';
	@Output('clear') clearEndpoint = new EventEmitter<any>();
	@Output('set') setEndpoint = new EventEmitter<any>();
	@Output('setEndpoint') setSelectedEndpoint = new EventEmitter<any>();
	loadingFilterEndpoints = false;
	endpoints$!: Observable<ENDPOINT[]>;
	selectedEndpoint!: ENDPOINT;

	constructor(public privateService: PrivateService) {}

	ngOnInit(): void {}

	ngAfterViewInit() {
		this.endpoints$ = fromEvent<any>(this.eventDelsEndpointFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getEndpointsForFilter(search))
		);
	}

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
}
