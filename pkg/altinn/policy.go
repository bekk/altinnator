package altinn

import "strings"

func maskinportenSchemaPolicy(id string) string {
	return strings.ReplaceAll(maskinportenSchemaPolicyTemplate, "{id}", id)
}

//nolint:lll
const maskinportenSchemaPolicyTemplate = `<?xml version="1.0" encoding="utf-8"?>
<xacml:Policy xmlns:xacml="urn:oasis:names:tc:xacml:3.0:core:schema:wd-17" PolicyId="urn:maskinportenschema:{id}:1" Version="1.0" RuleCombiningAlgId="urn:oasis:names:tc:xacml:3.0:rule-combining-algorithm:deny-overrides">
  <xacml:Target />
  <xacml:Rule RuleId="urn:maskinportenschema:{id}:1:1" Effect="Permit">
    <xacml:Description>MaskinportenSchema resource policy for; {id} for access package; maskinporten-scopes to have access to actions; ScopeAccess</xacml:Description>
    <xacml:Target>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:3.0:function:string-equal-ignore-case">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">maskinporten-scopes</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:altinn:accesspackage" Category="urn:oasis:names:tc:xacml:1.0:subject-category:access-subject" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:1.0:function:string-equal">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">{id}</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:altinn:resource" Category="urn:oasis:names:tc:xacml:3.0:attribute-category:resource" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
      <xacml:AnyOf>
        <xacml:AllOf>
          <xacml:Match MatchId="urn:oasis:names:tc:xacml:3.0:function:string-equal-ignore-case">
            <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#string">ScopeAccess</xacml:AttributeValue>
            <xacml:AttributeDesignator AttributeId="urn:oasis:names:tc:xacml:1.0:action:action-id" Category="urn:oasis:names:tc:xacml:3.0:attribute-category:action" DataType="http://www.w3.org/2001/XMLSchema#string" MustBePresent="false" />
          </xacml:Match>
        </xacml:AllOf>
      </xacml:AnyOf>
    </xacml:Target>
  </xacml:Rule>
  <xacml:ObligationExpressions>
    <xacml:ObligationExpression FulfillOn="Permit" ObligationId="urn:maskinportenschema:{id}:obligation:1">
      <xacml:AttributeAssignmentExpression AttributeId="urn:maskinportenschema:{id}:obligation-assignment:1" Category="urn:altinn:minimum-authenticationlevel">
        <xacml:AttributeValue DataType="http://www.w3.org/2001/XMLSchema#integer">3</xacml:AttributeValue>
      </xacml:AttributeAssignmentExpression>
    </xacml:ObligationExpression>
  </xacml:ObligationExpressions>
</xacml:Policy>`
