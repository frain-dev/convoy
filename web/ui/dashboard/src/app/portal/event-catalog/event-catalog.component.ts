import { Component, EventEmitter, Input, type OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from '../../components/button/button.component';
import {
    EventDeliveryDetailsModule
} from '../../private/pages/project/events/event-delivery-details/event-delivery-details.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { PrivateService } from '../../private/private.service';
import { PrismModule } from '../../private/components/prism/prism.module';
import { TagComponent } from '../../components/tag/tag.component';
import { GeneralService } from '../../services/general/general.service';
import { CardComponent } from '../../components/card/card.component';

export interface EventType {
    uid: string
    name: string
    category: string
    json_schema: any
    description: string
    deprecated_at: string | null
    expanded?: boolean // Track expanded/collapsed state
    parsed_schema?: any
    example_json?: any
    fields: Field[]
}

interface Field {
    name: string
    displayName: string
    type: string
    optional: boolean
    depth: number
    expanded: boolean
    children: Field[]
    description?: string
}

@Component({
    selector: "convoy-event-catalog",
    templateUrl: "./event-catalog.component.html",
    standalone: true,
    imports: [
        CommonModule,
        RouterModule,
        ButtonComponent,
        EventDeliveryDetailsModule,
        FormsModule,
        ReactiveFormsModule,
        PrismModule,
        TagComponent,
        CardComponent,
    ],
    styleUrls: ["./event-catalog.component.scss"],
})
export class EventCatalogComponent implements OnInit {
    @Input() singleEventMode: boolean = false;
    @Input() selectedEventType: any;
    @Output() eventTypesFetched = new EventEmitter<EventType[]>();
    eventTypes: EventType[] = [];

    filteredEventTypes = [...this.eventTypes];
    eventSearchString = ""
    isLoadingEvents = false

    portalToken = this.route.snapshot.queryParams.token

    constructor(
        private route: ActivatedRoute,
        private router: Router,
        public privateService: PrivateService,
        public generalService: GeneralService,
    ) {}

    async ngOnInit() {
        await this.getEventTypes();
    }

    fieldDescriptions: { [key: string]: string } = {
        status: "Indicates the current state of the entity.",
        message: "A textual message providing additional context.",
        data: "The primary data object containing relevant information.",
        assets: "A collection of digital or physical assets associated with the entity.",
        integration: "An identifier for system or service integration.",
        name: "A human-readable name for the entity.",
        title: "The title or heading associated with the entity.",
        description: "A detailed description explaining the entity.",
        summary: "A brief overview of the entity.",
        details: "Additional detailed information.",
        notes: "Additional remarks or comments.",
        code: "A unique identifier or reference code.",
        key: "A key used for identification or access.",
        value: "The associated value for a key or property.",
        price: "The monetary value of the entity, if applicable.",
        amount: "A numerical value representing an amount.",
        currency: "The currency in which the value is represented.",
        quantity: "The amount or count of the entity.",
        count: "The total number of occurrences.",
        total: "The aggregate or sum of values.",
        type: "The classification or category of the entity.",
        category: "The group or classification this entity belongs to.",
        group: "A related set of entities.",
        tag: "A label used for categorization or filtering.",
        tags: "A collection of labels for categorization.",
        files: "A list of files associated with the entity.",
        file_path: "The location or reference to a file.",
        url: "A web address related to the entity.",
        link: "A hyperlink pointing to an external or internal resource.",
        reference: "An external or internal reference ID.",
        parent_id: "The unique identifier of a related parent entity.",
        child_id: "The unique identifier of a related child entity.",
        active: "Indicates whether the entity is active or enabled.",
        enabled: "Specifies if a feature or setting is turned on.",
        disabled: "Specifies if a feature or setting is turned off.",
        locked: "Indicates whether the entity is locked from changes.",
        permissions: "A set of access rights or rules.",
        role: "The role associated with an entity.",
        owner: "The user or entity that owns this resource.",
        assigned_to: "The entity or user responsible for this item.",
        responsible_party: "The primary contact or owner of the entity.",
        features: "A set of characteristics or functionalities.",
        attributes: "Properties that define the entity.",
        settings: "Configuration options for the entity.",
        metadata: "Additional contextual data about the entity.",
        config: "A set of configuration parameters.",
        version: "The version number of the entity.",
        revision: "A specific revision or update reference.",
        slug: "A URL-friendly identifier.",
        redirect_url: "A URL to redirect to after an action.",
        notification_emails: "A list of emails for notifications.",
        phone_number: "A contact phone number.",
        email: "An email address associated with the entity.",
        ip_address: "An IP address associated with the entity.",
        location: "The geographical location of the entity.",
        address: "A physical or mailing address.",
        city: "The city associated with the entity.",
        state: "The state or region of the entity.",
        country: "The country associated with the entity.",
        expires_in: "The duration until expiration.",
        expiration_date: "The specific date and time of expiration.",
        id: "A unique identifier for the entity.",
        uuid: "A universally unique identifier.",
        reference_id: "An external reference identifier.",
        createdAt: "The timestamp when the entity was created.",
        updatedAt: "The timestamp when the entity was last updated.",
        deletedAt: "The timestamp when the entity was deleted, if applicable.",
        last_modified: "The last modification date and time.",
        start_date: "The date when an event or action begins.",
        end_date: "The date when an event or action ends.",
        duration: "The total time span of an event or process.",
        priority: "The importance level of the entity.",
        status_code: "A numerical code representing the status.",
        response_code: "A code representing the result of an operation.",
        error_code: "A specific code representing an error.",
        reason: "An explanation for a given action or status.",
        success: "Indicates whether the operation was successful.",
        failed: "Indicates whether the operation failed.",
        retry_count: "The number of times an action has been retried.",
        attempt: "A specific instance of trying an action.",
        retries_remaining: "The number of retry attempts left.",
        log_level: "The severity level of a log entry.",
        timestamp: "A specific moment in time.",
        event: "A recorded occurrence within the system.",
        event_type: "The classification of the recorded event.",
        request_id: "A unique identifier for a specific request.",
        response_time: "The time taken to respond to a request.",
        execution_time: "The duration taken for execution.",
    };


    async getEventTypes() {
        try {
            this.isLoadingEvents = true
            console.log("Fetching event types...")
            const response = await this.privateService.getEventTypes()
            const data = response.data
            for (const [index, value] of data.entries()) {
                if (value.json_schema) {
                    try {
                        value.parsed_schema =
                            typeof value.json_schema === 'string'
                                ? JSON.parse(value.json_schema)
                                : JSON.parse(JSON.stringify(value.json_schema));

                        if (
                            value.parsed_schema
                            // && value?.parsed_schema?.$schema?.includes('://json-schema.org/')
                        ) {
                            value.fields = this.extractFields(
                                value.parsed_schema.properties || {},
                                value.parsed_schema.required || []
                            );

                            // Use the first example in the examples array for JSON preview
                            value.example_json = value.parsed_schema.examples?.[0] || value.parsed_schema.example || {};
                        } else {
                            console.warn(`Invalid JSON schema for event: ${value.name}`);
                        }
                    } catch (error) {
                        console.error(`Failed to parse JSON schema for event: ${value.name}`, error);
                    }
                }
            }
            this.eventTypes = data
                .filter((type: EventType) => !type.deprecated_at)
                .map((type: EventType) => ({ ...type, expanded: false }))
            this.eventTypesFetched.emit(this.eventTypes);

            this.restoreExpandedState();

            this.isLoadingEvents = false
        } catch (error) {
            console.error("Error loading event types:", error)
            this.isLoadingEvents = false
        }
    }

    extractFields(properties: any, requiredFields: string[], parentKey = "", depth = 0): Field[] {
        const fields: Field[] = [];

        Object.keys(properties).forEach((key) => {
            const field = properties[key];
            const fieldType = field.type || "unknown";
            const fieldName = parentKey ? `${parentKey}.${key}` : key;

            const fieldObj: Field = {
                name: fieldName,
                displayName: field.title || key,
                type: fieldType,
                optional: !requiredFields.includes(key),
                depth: depth,
                expanded: false,
                children: [],
                description: field.description || this.fieldDescriptions[key] || this.generateDescription(key, fieldType),
            };

            if (fieldType === "object" && field.properties) {
                fieldObj.children = this.extractFields(field.properties, field.required || [], fieldName, depth + 1);
            }

            fields.push(fieldObj);
        });

        return fields;
    }

    generateDescription(fieldName: string, fieldType: string): string {
        switch (fieldType) {
            case "string":
                return `Text value representing the ${fieldName.toLowerCase()}.`
            case "number":
                return `Numeric value for the ${fieldName.toLowerCase()}.`
            case "boolean":
                return fieldName.toLowerCase() === "status" ? "True/false flag indicating the status." : `True/false flag indicating the ${fieldName.toLowerCase()} status.`;
            case "array[]":
                return `List of ${fieldName.toLowerCase()} items.`
            case "object":
                return `Object containing ${fieldName.toLowerCase()} information.`
            default:
                return `The ${fieldName.toLowerCase()} of the item.`
        }
    }

    toggleEventTypeExpand(eventType: EventType) {
        eventType.expanded = !eventType.expanded;
        this.saveExpandedState();
    }

    toggleFieldExpand(field: Field) {
        field.expanded = !field.expanded
        this.saveExpandedState();
    }

    saveExpandedState() {
        const expandedEventIds = this.eventTypes
            .filter(eventType => eventType.expanded)
            .map(eventType => eventType.uid);

        const expandedFields = this.collectExpandedFields(this.eventTypes);

        localStorage.setItem('expandedEventTypes', JSON.stringify(expandedEventIds));
        localStorage.setItem('expandedFields', JSON.stringify(expandedFields));
    }

    collectExpandedFields(eventTypes: EventType[]): string[] {
        let expandedFields: string[] = [];

        eventTypes.forEach(eventType => {
            expandedFields.push(...this.collectExpandedFieldsRecursive(eventType.fields, eventType.uid));
        });

        return expandedFields;
    }

    collectExpandedFieldsRecursive(fields: Field[], parentPath: string): string[] {
        if (!fields) return [];
        let expanded: string[] = [];

        fields.forEach(field => {
            const fieldPath = `${parentPath}.${field.name}`; // Use full path for uniqueness
            if (field.expanded) {
                expanded.push(fieldPath);
            }
            if (field.children.length > 0) {
                expanded.push(...this.collectExpandedFieldsRecursive(field.children, fieldPath));
            }
        });

        return expanded;
    }

    restoreExpandedState() {
        const savedExpandedEventIds = JSON.parse(localStorage.getItem('expandedEventTypes') || '[]');
        const savedExpandedFields = JSON.parse(localStorage.getItem('expandedFields') || '[]');

        this.eventTypes.forEach(eventType => {
            eventType.expanded = savedExpandedEventIds.includes(eventType.uid);
            this.restoreExpandedFields(eventType.fields, eventType.uid, savedExpandedFields);
        });
    }

    restoreExpandedFields(fields: Field[], parentPath: string, savedExpandedFields: string[]) {
        if (!fields) return;
        fields.forEach(field => {
            const fieldPath = `${parentPath}.${field.name}`; // Use full path for matching
            if (savedExpandedFields.includes(fieldPath)) {
                field.expanded = true;
            }
            if (field.children.length > 0) {
                this.restoreExpandedFields(field.children, fieldPath, savedExpandedFields);
            }
        });
    }



    getFieldDescription(field: Field): string {
        return field.description || `Information about the ${field.displayName.toLowerCase()}.`
    }

    get displayedEventTypes() {
        if (!this.eventSearchString?.trim()) {
            return this.eventTypes;
        }
        const term = this.eventSearchString.toLowerCase();
        return this.eventTypes.filter(event =>
            event.name.toLowerCase().includes(term) ||
            event.description?.toLowerCase().includes(term) ||
            JSON.stringify(event.json_schema || {}).toLowerCase().includes(term)
        );
    }

    filterEvents(searchTerm: string): void {
        const term = searchTerm.toLowerCase();

        this.filteredEventTypes = this.eventTypes.filter((event) =>
            event.name.toLowerCase().includes(term) ||
            event.description?.toLowerCase().includes(term) ||
            JSON.stringify(event.json_schema || {}).toLowerCase().includes(term)
        );
    }


    getTagColor(type: string): 'primary' | 'error' | 'success' | 'warning' | 'neutral' {
        const colorMap: { [key: string]: 'primary' | 'error' | 'success' | 'warning' | 'neutral' } = {
            'array[]': 'warning',
            array: 'warning',
            boolean: 'success',
            number: 'error',
            object: 'primary',
        };
        return colorMap[type] || 'neutral';
    }
}
