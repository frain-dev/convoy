import {Component, EventEmitter, Input, OnInit, Output, ViewChild} from '@angular/core';
import {CommonModule} from '@angular/common';
import {CardComponent} from 'src/app/components/card/card.component';
import {FormBuilder, ReactiveFormsModule} from '@angular/forms';
import {ButtonComponent} from 'src/app/components/button/button.component';
import {CreateSubscriptionService} from '../create-subscription/create-subscription.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {MonacoComponent} from '../monaco/monaco.component';
import {ActivatedRoute} from '@angular/router';
import {DialogHeaderComponent} from 'src/app/components/dialog/dialog.directive';

@Component({
    selector: 'convoy-create-subscription-filter',
    imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent, DialogHeaderComponent],
    templateUrl: './create-subscription-filter.component.html',
    styleUrls: ['./create-subscription-filter.component.scss']
})
export class CreateSubscriptionFilterComponent implements OnInit {
    @ViewChild('requestHeaderEditor') requestHeaderEditor!: MonacoComponent;
    @ViewChild('headerSchemaEditor') headerSchemaEditor!: MonacoComponent;
    @ViewChild('requestEditor') requestEditor!: MonacoComponent;
    @ViewChild('schemaEditor') schemaEditor!: MonacoComponent;
    @ViewChild('requestQueryEditor') requestQueryEditor!: MonacoComponent;
    @ViewChild('querySchemaEditor') querySchemaEditor!: MonacoComponent;
    @ViewChild('requestPathEditor') requestPathEditor!: MonacoComponent;
    @ViewChild('pathSchemaEditor') pathSchemaEditor!: MonacoComponent;

    @Input('action') action: 'update' | 'create' | 'view' | 'portal' = 'create';
    @Input('schema') schema?: any;
    @Input('selectedEventType') selectedEventType?: string = '';

    @Output('close') close: EventEmitter<any> = new EventEmitter();
    @Output('filterSchema') filterSchema: EventEmitter<any> = new EventEmitter();

    dialogName = 'Event Filter';
    isLoading = false;

    tabs: ['body', 'headers', 'query', 'path'] = ['body', 'headers', 'query', 'path'];
    activeTab: 'body' | 'headers' | 'query' | 'path' = 'body';
    isFilterTestPassed = false;
    payload: any = {
        id: 'Sample-1',
        name: 'Sample 1',
        description: 'This is sample data #1'
    };
    header: any = {
        'X-Gitlab-Event': 'Push Hook'
    };
    query: any = {
        event_type: 'push'
    };
    path: any = {
        path: '/ingest/source-id'
    };

    constructor(private formBuilder: FormBuilder, private createSubscriptionService: CreateSubscriptionService, private generalService: GeneralService, private route: ActivatedRoute) {}

    ngOnInit() {
        this.checkForExistingData();
        this.dialogName = this.dialogName + ' for "' + this.selectedEventType + '"';
    }

    toggleActiveTab(tab: 'body' | 'headers' | 'query' | 'path') {
        this.activeTab = tab;
    }

    async testFilter() {
        this.isFilterTestPassed = false;

        const testVals = {
            request: {
                header: this.requestHeaderEditor?.getValue() ? this.generalService.convertStringToJson(this.requestHeaderEditor.getValue()) : null,
                body: this.requestEditor?.getValue() ? this.generalService.convertStringToJson(this.requestEditor.getValue()) : null,
                query: this.requestQueryEditor?.getValue() ? this.generalService.convertStringToJson(this.requestQueryEditor.getValue()) : null,
                path: this.requestPathEditor?.getValue() ? this.generalService.convertStringToJson(this.requestPathEditor.getValue()) : null
            },
            schema: {
                header: this.headerSchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.headerSchemaEditor.getValue()) : null,
                body: this.schemaEditor?.getValue() ? this.generalService.convertStringToJson(this.schemaEditor.getValue()) : null,
                query: this.querySchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.querySchemaEditor.getValue()) : null,
                path: this.pathSchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.pathSchemaEditor.getValue()) : null
            }
        };

        try {
            const response = await this.createSubscriptionService.testSubscriptionFilter(testVals);
            const testResponse = `The sample data was ${!response.data ? 'not' : ''} accepted by the filter`;
            this.generalService.showNotification({message: testResponse, style: !response.data ? 'error' : 'success'});
            this.isFilterTestPassed = !!response.data;
            return this.isFilterTestPassed;
        } catch (error) {
            this.isFilterTestPassed = false;
            return error;
        }
    }

    async setSubscriptionFilter() {
        await this.testFilter();

        if (this.isFilterTestPassed) {
            if (this.requestEditor?.getValue()) localStorage.setItem('EVENT_DATA', this.requestEditor.getValue());
            if (this.requestHeaderEditor?.getValue()) localStorage.setItem('EVENT_HEADERS', this.requestHeaderEditor.getValue());
            if (this.requestQueryEditor?.getValue()) localStorage.setItem('EVENT_QUERY', this.requestQueryEditor.getValue());
            if (this.requestPathEditor?.getValue()) localStorage.setItem('EVENT_PATH', this.requestPathEditor.getValue());
            const filter = {
                bodySchema: this.schemaEditor?.getValue() ? this.generalService.convertStringToJson(this.schemaEditor?.getValue()) : null,
                headerSchema: this.headerSchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.headerSchemaEditor?.getValue()) : null,
                querySchema: this.querySchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.querySchemaEditor?.getValue()) : null,
                pathSchema: this.pathSchemaEditor?.getValue() ? this.generalService.convertStringToJson(this.pathSchemaEditor?.getValue()) : null
            };
            this.filterSchema.emit(filter);
        }
    }

    checkForExistingData() {
        const eventData = localStorage.getItem('EVENT_DATA');
        const eventHeaders = localStorage.getItem('EVENT_HEADERS');
        const eventQuery = localStorage.getItem('EVENT_QUERY');
        const eventPath = localStorage.getItem('EVENT_PATH');
        if (eventData && eventData !== 'undefined') this.payload = JSON.parse(eventData);
        if (eventHeaders && eventHeaders !== 'undefined') this.header = JSON.parse(eventHeaders);
        if (eventQuery && eventQuery !== 'undefined') this.query = JSON.parse(eventQuery);
        if (eventPath && eventPath !== 'undefined') this.path = JSON.parse(eventPath);
    }
}
